package main

import (
	"os"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/cmd/execute"
)

func main() {
	execute.Execute(os.Args)
}
