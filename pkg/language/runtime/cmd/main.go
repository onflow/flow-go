package main

import (
	"os"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/cmd/execute"
)

func main() {
	execute.Execute(os.Args[1:])
}
