package cmd

import (
	"github.com/dapperlabs/bamboo-node/language/runtime"
	. "github.com/dapperlabs/bamboo-node/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/language/runtime/parser"
	"fmt"
	"io/ioutil"
	"os"
)

// main parses the given filename and prints any syntax errors.
// if there are no syntax errors, the program is interpreted.
// if after the interpretation a global function `main` is defined, it will be called.
// the program may call the function `log` to print a value.
//
func main() {

	if len(os.Args) < 2 {
		exitWithError("no input file")
	}
	filename := os.Args[1]

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		exitWithError(err.Error())
	}
	code := string(data)

	program, errors := parser.Parse(code)
	if len(errors) > 0 {
		for _, err := range errors {
			prettyPrintError(err, filename, code)
		}
		os.Exit(1)
	}

	inter := NewInterpreter(program)
	inter.ImportFunction(
		"log",
		NewHostFunction(
			&FunctionType{
				ParameterTypes: []Type{&AnyType{}},
				ReturnType:     &VoidType{},
			},
			func(_ *Interpreter, arguments []Value) Value {
				fmt.Printf("%v\n", arguments[0])
				return &VoidValue{}
			},
		),
	)

	err = inter.Interpret()
	if err != nil {
		prettyPrintError(err, filename, code)
		os.Exit(1)
	}

	if _, hasMain := inter.Globals["main"]; !hasMain {
		return
	}

	_, err = inter.Invoke("main")
	if err != nil {
		prettyPrintError(err, filename, code)
		os.Exit(1)
	}
}

func prettyPrintError(err error, filename string, code string) {
	print(strictus.PrettyPrintError(err, filename, code, true))
}

func exitWithError(message string) {
	print(strictus.FormatErrorMessage(message, true))
	os.Exit(1)
}
