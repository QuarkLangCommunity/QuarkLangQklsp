# QuarkLangQklsp

> **Superseded. The maintained implementation** is
> [`cmd/qklsp`](https://github.com/QuarkLangCommunity/QuarkLangQkc/tree/main/cmd/qklsp)
> in the QuarkLang compiler repository. This standalone repository is kept for history; it is not
> updated from that source, and its code differs from the maintained implementation.

QuarkLang 语言服务器（LSP over stdio）+ VSCode 扩展——工具链成员。

## 能力

| 能力 | 说明 |
|---|---|
| **诊断** | 打开/修改文件即用解释器同款前端（parse + typecheck）检查，错误按 `line/col` 转成 LSP Diagnostic |
| **文档符号** | `textDocument/documentSymbol`：函数 / 类型 / 接口 / impl |
| **悬停** | `textDocument/hover`：所在行的函数签名 |

## 用法

```sh
go build -o qklsp .        # go.mod 里 replace 指向 ../QuarkLang
qklsp                      # 编辑器以 stdio 启动
```

实测（脚本驱动一次会话）：

```
INIT reply: ['documentSymbolProvider', 'hoverProvider', 'textDocumentSync']
DIAG: 1 line 2 col 12 | ParseError: unexpected token ';' in expression at line 3, col 13
SYMBOLS: []
```

（诊断位置 2:12 为 0 基，对应源码第 3 行 13 列 ✓；有语法错误时 AST 不可用，符号列表为空。）

## VSCode 扩展

`vscode/` 是一个可直接 `vsce package` 的扩展骨架：

- `syntaxes/quarklang.tmLanguage.json`：TextMate 语法（关键字/类型/运算符方法名 `__add__`/字符串含原始字符串/数字/注释/函数与类型名/运算符）
- `language-configuration.json`：注释、括号、自动闭合、缩进规则
- `package.json`：注册 `.qk` 语言与语法；接 LSP 时把 `qklsp` 作为 server 启动即可

安装（开发态）：`ln -s "$PWD/vscode" ~/.vscode/extensions/quarklang-0.1.0`
