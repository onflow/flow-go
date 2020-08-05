<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Language Server Protocol Go Types](#language-server-protocol-go-types)
  - [Setup](#setup)
  - [Generate](#generate)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Language Server Protocol Go Types

### Setup

- `npm install -g ts-node`
- `npm install`
- `curl https://raw.githubusercontent.com/golang/tools/master/internal/lsp/protocol/typescript/go.ts --output generate.ts`
- `git clone https://github.com/microsoft/vscode-languageserver-node.git`

### Generate

- `ts-node generate.ts -d . -o ../protocol/types.go`
- `gofmt -w ../protocol/types.go`
