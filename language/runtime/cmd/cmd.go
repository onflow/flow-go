package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
)

func PrettyPrintError(err error, filename string, codes map[string]string) {
	i := 0
	printErr := func(err error, filename string) {
		if i > 0 {
			println()
		}
		print(runtime.PrettyPrintError(err, filename, codes[filename], true))
		i++
	}

	if parserError, ok := err.(parser.Error); ok {
		for _, err := range parserError.Errors {
			printErr(err, filename)
		}
	} else if checkerError, ok := err.(*sema.CheckerError); ok {
		for _, err := range checkerError.Errors {
			printErr(err, filename)
			if err, ok := err.(*sema.ImportedProgramError); ok {
				filename := string(err.ImportLocation.(ast.StringLocation))
				for _, err := range err.CheckerError.Errors {
					PrettyPrintError(err, filename, codes)
				}
			}
		}
	} else if locatedErr, ok := err.(ast.HasImportLocation); ok {
		location := locatedErr.ImportLocation()
		if location != nil {
			filename = string(location.(ast.StringLocation))
		}
		printErr(err, filename)
	} else {
		printErr(err, filename)
	}
}

func must(err error, filename string, codes map[string]string) {
	if err == nil {
		return
	}
	PrettyPrintError(err, filename, codes)
	os.Exit(1)
}

func mustClosure(filename string, codes map[string]string) func(error) {
	return func(e error) {
		must(e, filename, codes)
	}
}

//PrepareChecker prepares and initializes a Checker with a given code as a string
//and dummyFilename which is used for pretty-printing errors, if any
func PrepareChecker(code string, dummyFilename string) (*sema.Checker, func(error)) {
	codes := map[string]string{}

	must := mustClosure(dummyFilename, codes)

	program, _, err := parser.ParseProgram(code)
	codes[dummyFilename] = code
	must(err)

	err = program.ResolveImports(func(location ast.Location) (program *ast.Program, err error) {
		switch location := location.(type) {
		case ast.StringLocation:
			filename := string(location)
			imported, _, code, err := parser.ParseProgramFromFile(filename)
			codes[filename] = code
			must(err)
			return imported, nil

		default:
			return nil, fmt.Errorf("cannot import `%s`. only files are supported", location)
		}
	})
	must(err)

	standardLibraryFunctions := standardLibraryFunctions()
	valueDeclarations := standardLibraryFunctions.ToValueDeclarations()
	typeDeclarations := stdlib.BuiltinTypes.ToTypeDeclarations()

	location := runtime.FileLocation(dummyFilename)
	checker, err := sema.NewChecker(
		program,
		location,
		sema.WithPredeclaredValues(valueDeclarations),
		sema.WithPredeclaredTypes(typeDeclarations),
	)
	must(err)

	must(checker.Check())

	return checker, must
}

func standardLibraryFunctions() stdlib.StandardLibraryFunctions {
	return append(stdlib.BuiltinFunctions, stdlib.HelperFunctions...)
}

func PrepareInterpreter(filename string) (*interpreter.Interpreter, *sema.Checker, func(error)) {

	codeBytes, err := ioutil.ReadFile(filename)

	checker, must := PrepareChecker(string(codeBytes), filename)

	values := standardLibraryFunctions().ToValues()

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(values),
	)
	must(err)

	must(inter.Interpret())

	return inter, checker, func(err error) {
		must(err)
	}
}

func ExitWithError(message string) {
	print(runtime.FormatErrorMessage(message, true))
	os.Exit(1)
}
