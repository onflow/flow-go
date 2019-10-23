package main

import (
	"os"

	"github.com/dapperlabs/flow-go/language/runtime/cmd/execute"
)

func main() {
	execute.Execute(os.Args[1:])
}
