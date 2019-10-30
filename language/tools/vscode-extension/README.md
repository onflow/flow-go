# Cadence Visual Studio Code Extension

## Features

- Syntax highlighting (including in Markdown code fences)

## Installation

To install the extension, ensure you [have VS Code installed](https://code.visualstudio.com/docs/setup/mac)
and have configured the [`code` command line interface](https://code.visualstudio.com/docs/setup/mac#_launching-from-the-command-line),
then run:
```shell script
code --install-extension cadence-*.vsix
```

The `cadence-*.vsix` is a pre-built extension package included in this repo.

### Language Server
Once the extension is installed, it needs to be configured with the language server.

#### Using the Flow CLI
The recommended way to do this is to install the [Flow CLI](TODO) via Homebrew.
The default configuration of the extension assumes the CLI is installed this way,
so if you do this, you're done!

## Building
If you are making changes to the extension, you will need to manually build it.

Ensure you have `npm` installed, then run:
```shell script
npm install
npm install -g vsce
vsce package
code --install-extension cadence-*.vsix
```

## Configuration
The extension allows configuring the command used to start the language server:
- Open Visual Studio Code
- Go to `Preferences` → `Settings`
  - Search for "Cadence"
  - In "Cadence: Language Server Command", enter the command to start the language
    server (the default is `flow cadence language-server`).
- Open or create a new `.cdc` file
- The language mode should be set to `Cadence` automatically
- A popup should appear "Cadence language server started"
- Happy coding!

## Commands
- "Cadence: Restart language server"
