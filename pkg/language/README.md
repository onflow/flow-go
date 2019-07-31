# Bamboo Programming Language

This package contains everything related to the programming language:

  - [Documentation](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/language/docs)
  - [Interpreter](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/language/runtime)
  - Tools
    - [Visual Studio Code Extension](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/language/tools/vscode-extension)

## Status

- The language is still under design and the current decisions can be found in the
  [documentation](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/language/docs)

- Work on an [interpreter](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/language/runtime)
  has started. It only implements a small subset of the language, but is already embedded in the emulator

- We plan to write a specification for the language in [K](http://www.kframework.org/index.php/Main_Page)
  and automatically derive an interpreter from it

- Once we have received more feedack for the first iteration of the language and its implementation,
  we hope to specify an instruction set and implement a more efficient virtual machine
