## Language Server Protocol Go Types

### Setup

- `npm install -g ts-node`
- `npm install`
- `curl https://raw.githubusercontent.com/golang/tools/master/internal/lsp/protocol/typescript/go.ts --output generate.ts`
- `git clone https://github.com/microsoft/vscode-languageserver-node.git`

### Generate

- `ts-node generate.ts -d . -o ../protocol/types.go`
- `gofmt -w ../protocol/types.go`
