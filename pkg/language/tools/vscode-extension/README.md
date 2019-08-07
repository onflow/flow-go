# Bamboo Visual Studio Code Extension

## Features

- Syntax highlighting (including in Markdown code fences)

## Installation

- `npm install`
- `npm install -g vsce`
- `vsce package`
- `code --install-extension bamboo-*.vsix`
- Open Visual Studio Code
- Go to `Preferences` → `Settings`
  - Search for "Bamboo"
  - In "Bamboo: Language Server Path", enter the full path to `bamboo-node/pkg/language/tools/language-server`
- Open or create a new `.bpl` file
- The language mode should be set to `Bamboo` automatically
- A popup should appear "Bamboo language server started"
- Happy coding!
