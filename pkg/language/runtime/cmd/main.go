package main

import (
	"io/ioutil"
	"os"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/stdlib"
)

// main parses the given filename and prints any syntax errors.
// if there are no syntax errors, the program is interpreted.
// if after the interpretation a global function `main` is defined, it will be called.
// the program may call the function `log` to print a value.
//
func main() {

	standardLibraryFunctions := append(stdlib.BuiltIns, stdlib.Helpers...)

	if len(os.Args) < 2 {
		exitWithError("no input file")
	}
	filename := os.Args[1]

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		exitWithError(err.Error())
	}
	code := string(data)

	program, errors := parser.ParseProgram(code)
	if len(errors) > 0 {
		for _, err := range errors {
			prettyPrintError(err, filename, code)
		}
		os.Exit(1)
	}

	checker := sema.NewChecker(program)
	for _, function := range standardLibraryFunctions {
		if err = checker.DeclareValue(function); err != nil {
			prettyPrintError(err, filename, code)
			os.Exit(1)
		}
	}

	err = checker.Check()
	if err != nil {
		prettyPrintError(err, filename, code)
		os.Exit(1)
	}

	inter := interpreter.NewInterpreter(checker)
	for _, function := range standardLibraryFunctions {
		err = inter.ImportFunction(function.Name, function.Function)
		if err != nil {
			prettyPrintError(err, filename, code)
			os.Exit(1)
		}
	}

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
	var errs []error
	if checkerError, ok := err.(*sema.CheckerError); ok {
		errs = checkerError.Errors
	} else {
		errs = []error{err}
	}

	for i, err := range errs {
		if i > 0 {
			println()
		}
		print(runtime.PrettyPrintError(err, filename, code, true))
	}
}

func exitWithError(message string) {
	print(runtime.FormatErrorMessage(message, true))
	os.Exit(1)
}
