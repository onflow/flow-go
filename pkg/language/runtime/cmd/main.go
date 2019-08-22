package main

import (
	"fmt"
	"os"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
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

	standardLibraryFunctions := append(stdlib.BuiltinFunctions, stdlib.HelperFunctions...)

	if len(os.Args) < 2 {
		exitWithError("no input file")
	}
	filename := os.Args[1]

	codes := map[string]string{}

	must := func(err error, filename string) {
		if err == nil {
			return
		}
		prettyPrintError(err, filename, codes)
		os.Exit(1)
	}

	program, code, err := parser.ParseProgramFromFile(filename)
	codes[filename] = code
	must(err, filename)

	err = program.ResolveImports(func(location ast.ImportLocation) (program *ast.Program, err error) {
		switch location := location.(type) {
		case ast.StringImportLocation:
			filename := string(location)
			imported, code, err := parser.ParseProgramFromFile(filename)
			codes[filename] = code
			must(err, filename)
			return imported, nil

		default:
			return nil, fmt.Errorf("cannot import `%s`. only files are supported", location)
		}
	})
	must(err, filename)

	valueDeclarations := stdlib.ToValueDeclarations(standardLibraryFunctions)

	checker, err := sema.NewChecker(program, valueDeclarations, nil)
	must(err, filename)

	must(checker.Check(), filename)

	values := stdlib.ToValues(standardLibraryFunctions)

	inter, err := interpreter.NewInterpreter(checker, values)
	must(err, filename)

	must(inter.Interpret(), filename)

	if _, hasMain := inter.Globals["main"]; !hasMain {
		return
	}

	_, err = inter.Invoke("main")
	must(err, filename)
}

func prettyPrintError(err error, filename string, codes map[string]string) {
	i := 0
	printErr := func(err error, filename string) {
		if i > 0 {
			println()
		}
		print(runtime.PrettyPrintError(err, filename, codes[filename], true))
		i += 1
	}

	if parserError, ok := err.(parser.Error); ok {
		for _, err := range parserError.Errors {
			printErr(err, filename)
		}
	} else if checkerError, ok := err.(*sema.CheckerError); ok {
		for _, err := range checkerError.Errors {
			printErr(err, filename)
			if err, ok := err.(*sema.ImportedProgramError); ok {
				filename := string(err.ImportLocation.(ast.StringImportLocation))
				for _, err := range err.CheckerError.Errors {
					prettyPrintError(err, filename, codes)
				}
			}
		}
	} else if locatedErr, ok := err.(ast.HasImportLocation); ok {
		filename := string(locatedErr.ImportLocation().(ast.StringImportLocation))
		printErr(err, filename)
	} else {
		printErr(err, filename)
	}
}

func exitWithError(message string) {
	print(runtime.FormatErrorMessage(message, true))
	os.Exit(1)
}
