package main

import (
	"os"

	"github.com/dapperlabs/flow-go/language/runtime/cmd/abi"
	"github.com/dapperlabs/flow-go/language/runtime/cmd/execute"
)

func main() {
	if len(os.Args) > 1 {
		//execute.Execute(os.Args[1:])
		abi.GenerateABI(os.Args[1:])
	} else {
		execute.RunREPL()
	}
}
