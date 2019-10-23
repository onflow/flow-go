# Bamboo Visual Studio Code Extension

## Features

- Syntax highlighting (including in Markdown code fences)

## Installation

First, build the language server. This will build the language server and place
it in `$GOPATH/bin/language-server`.
- `cd flow-go/language/tools/language-server`
- `GO111MODULE=on go install`

- `npm install`
- `npm install -g vsce`
- `vsce package`
- `code --install-extension bamboo-*.vsix`
- Open Visual Studio Code
- Go to `Preferences` → `Settings`
  - Search for "Bamboo"
  - In "Bamboo: Language Server Path", enter the binary path for the language
    server. If `$GOPATH/bin` is in your path, you can use `language-server`, if
    not use the full path to the binary instead (`$GOPATH/bin/language-server`).
- Open or create a new `.bpl` file
- The language mode should be set to `Bamboo` automatically
- A popup should appear "Bamboo language server started"
- Happy coding!

## Commands

- "Bamboo: Restart language server"
