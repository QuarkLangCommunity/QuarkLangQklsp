// qklsp: QuarkLang 语言服务器（LSP over stdio）——工具链成员
//
// 能力：诊断（复用解释器前端 parse+typecheck）、文档符号、悬停（函数签名）。
// 用法：编辑器以 stdio 启动 qklsp。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"quarklang/internal/lang"
)

type server struct {
	docs map[string]string
	out  *bufio.Writer
}

func (s *server) send(v interface{}) {
	body, _ := json.Marshal(v)
	fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n", len(body))
	s.out.Write(body)
	s.out.Flush()
}

func (s *server) reply(id interface{}, result interface{}) {
	if id == nil {
		return
	}
	s.send(map[string]interface{}{"jsonrpc": "2.0", "id": id, "result": result})
}

func (s *server) notify(method string, params interface{}) {
	s.send(map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params})
}

var (
	reFilePos = regexp.MustCompile("--> .*?:([0-9]+):([0-9]+)")
	reLineCol = regexp.MustCompile("line ([0-9]+), col ([0-9]+)")
	reLine    = regexp.MustCompile("line ([0-9]+)")
)

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// diagnostics 用解释器前端检查，并把错误位置转成 LSP 诊断。
func diagnostics(text, path string) []interface{} {
	_, err := lang.CompileWithImports(text, path)
	out := []interface{}{}
	if err == nil {
		return out
	}
	msg := err.Error()
	line, col := 1, 1
	if m := reFilePos.FindStringSubmatch(msg); m != nil {
		line, _ = strconv.Atoi(m[1])
		col, _ = strconv.Atoi(m[2])
	} else if m := reLineCol.FindStringSubmatch(msg); m != nil {
		line, _ = strconv.Atoi(m[1])
		col, _ = strconv.Atoi(m[2])
	} else if m := reLine.FindStringSubmatch(msg); m != nil {
		line, _ = strconv.Atoi(m[1])
	}
	clean := strings.TrimPrefix(msg, "error: ")
	out = append(out, map[string]interface{}{
		"range": map[string]interface{}{
			"start": map[string]int{"line": maxi(0, line-1), "character": maxi(0, col-1)},
			"end":   map[string]int{"line": maxi(0, line-1), "character": maxi(0, col)},
		},
		"severity": 1,
		"source":   "qklsp",
		"message":  clean,
	})
	return out
}

func symbols(text string) []interface{} {
	out := []interface{}{}
	prog, err := lang.Compile(text)
	if err != nil || prog == nil {
		return out
	}
	add := func(name string, kind int, line int) {
		rng := map[string]interface{}{
			"start": map[string]int{"line": maxi(0, line-1), "character": 0},
			"end":   map[string]int{"line": maxi(0, line-1), "character": 0},
		}
		out = append(out, map[string]interface{}{
			"name": name, "kind": kind, "range": rng, "selectionRange": rng,
		})
	}
	for _, f := range prog.Funcs {
		add(f.Name, 12, f.Pos.Line)
	}
	for _, st := range prog.Structs {
		add(st.Name, 5, st.Pos.Line)
	}
	for _, it := range prog.Interfaces {
		add(it.Name, 11, it.Pos.Line)
	}
	for _, im := range prog.Impls {
		add("impl "+im.Type, 5, im.Pos.Line)
	}
	return out
}

// nav 从嵌套 map 里按路径取值。
func nav(m map[string]interface{}, path ...string) interface{} {
	var cur interface{} = m
	for _, k := range path {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func asInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func main() {
	in := bufio.NewReader(os.Stdin)
	s := &server{docs: map[string]string{}, out: bufio.NewWriter(os.Stdout)}
	for {
		length := -1
		for {
			line, err := in.ReadString('\n')
			if err != nil {
				_ = io.EOF
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				length, _ = strconv.Atoi(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
			}
		}
		if length < 0 {
			continue
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(in, buf); err != nil {
			return
		}
		var m map[string]interface{}
		if err := json.Unmarshal(buf, &m); err != nil {
			continue
		}
		method := asString(m["method"])
		id := m["id"]
		params, _ := m["params"].(map[string]interface{})
		switch method {
		case "initialize":
			s.reply(id, map[string]interface{}{
				"capabilities": map[string]interface{}{
					"textDocumentSync":       1,
					"documentSymbolProvider": true,
					"hoverProvider":          true,
				},
				"serverInfo": map[string]string{"name": "qklsp", "version": "0.1.0"},
			})
		case "initialized":
		case "shutdown":
			s.reply(id, nil)
		case "exit":
			return
		case "textDocument/didOpen", "textDocument/didChange":
			uri := asString(nav(params, "textDocument", "uri"))
			text := asString(nav(params, "textDocument", "text"))
			if changes, ok := params["contentChanges"].([]interface{}); ok && len(changes) > 0 {
				last, _ := changes[len(changes)-1].(map[string]interface{})
				text = asString(last["text"])
			}
			s.docs[uri] = text
			path := strings.TrimPrefix(uri, "file://")
			s.notify("textDocument/publishDiagnostics", map[string]interface{}{
				"uri": uri, "diagnostics": diagnostics(text, path),
			})
		case "textDocument/didClose":
			uri := asString(nav(params, "textDocument", "uri"))
			delete(s.docs, uri)
			s.notify("textDocument/publishDiagnostics", map[string]interface{}{
				"uri": uri, "diagnostics": []interface{}{},
			})
		case "textDocument/documentSymbol":
			uri := asString(nav(params, "textDocument", "uri"))
			text := s.docs[uri]
			if text == "" {
				if b, err := os.ReadFile(strings.TrimPrefix(uri, "file://")); err == nil {
					text = string(b)
				}
			}
			s.reply(id, symbols(text))
		case "textDocument/hover":
			uri := asString(nav(params, "textDocument", "uri"))
			lineNo := asInt(nav(params, "position", "line")) + 1
			text := s.docs[uri]
			contents := "QuarkLang"
			if prog, err := lang.Compile(text); err == nil && prog != nil {
				for _, f := range prog.Funcs {
					if f.Pos.Line == lineNo {
						var ps []string
						for _, prm := range f.Params {
							ps = append(ps, prm.Type+" "+prm.Name)
						}
						ret := f.Ret
						if ret == "" {
							ret = "void"
						}
						contents = "fn " + f.Name + "(" + strings.Join(ps, ", ") + ") " + ret
						break
					}
				}
			}
			s.reply(id, map[string]interface{}{
				"contents": map[string]string{"kind": "plaintext", "value": contents},
			})
		default:
			if id != nil && method != "" {
				s.reply(id, nil)
			}
		}
	}
}
