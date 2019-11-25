package execute

import (
	"github.com/dapperlabs/flow-go/language/runtime/cmd"
)

// Execute parses the given filename and prints any syntax errors.
// If there are no syntax errors, the program is interpreted.
// If after the interpretation a global function `main` is defined, it will be called.
// The program may call the function `log` to print a value.
func Execute(args []string) {

	inter, _, must := cmd.PrepareInterpreter(args)

	if _, hasMain := inter.Globals["main"]; !hasMain {
		return
	}

	_, err := inter.Invoke("main")
	must(err)
}
