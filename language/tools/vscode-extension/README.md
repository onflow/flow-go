# Cadence Visual Studio Code Extension

## Features

- Syntax highlighting (including in Markdown code fences)
- Run the emulator, submit transactions, scripts from the editor

More details are [in Notion](https://www.notion.so/dapperlabs/Using-Eddy-fa9df1c81bde4a81a449286b162f821e)

## Installation

To install the extension, ensure you [have VS Code installed](https://code.visualstudio.com/docs/setup/mac)
and have configured the [`code` command line interface](https://code.visualstudio.com/docs/setup/mac#_launching-from-the-command-line).

### Using the Flow CLI

The recommended way to install the latest released version is to use the Flow CLI. 
```shell script
brew tap dapperlabs/homebrew && brew install flow-cli
```

Check that it's been installed correctly.
```shell script
flow version
```

Next, use the CLI to install the VS Code extension.
```shell script
flow cadence install-vscode-extension
```

Restart VS Code and the extension should be installed!

### Building

If you are building the extension from source, you need to build both the 
extension itself and the Flow CLI (if you don't already have a version installed).
Unless you're developing the extension or need access to unreleased features, 
you should use the Flow CLI option. It's much easier!

#### VS Code Extension
Make sure you are in this `vscode-extension` directory. 

If you haven't already, install dependencies.
```shell script
npm install
```

Next, build and package the extension.
```shell script
npm run package
```

This will result in a `.vsix` file containing the packaged extension. 

Install the packaged extension.
```shell script
code --install-extension cadence-*.vsix
```

Restart VS Code and the extension should be installed!

#### FLow CLI

Make sure you are in the root directory.

Build the Flow CLI.
```shell script
make cmd/flow/flow
```

Move the resulting binary (`cmd/flow/flow`) into your `$PATH`. For example:
```shell script
mv ./cmd/flow/flow /usr/local/bin/
```

Restart your terminal and check to ensure it was installed correctly.
```shell script
flow version
```

