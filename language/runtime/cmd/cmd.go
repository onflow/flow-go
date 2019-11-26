package cmd

import (
	"fmt"
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

func PrepareInterpreter(filename string) (*interpreter.Interpreter, *sema.Checker, func(error)) {

	codes := map[string]string{}

	must := func(err error, filename string) {
		if err == nil {
			return
		}
		PrettyPrintError(err, filename, codes)
		os.Exit(1)
	}

	program, _, code, err := parser.ParseProgramFromFile(filename)
	codes[filename] = code
	must(err, filename)

	err = program.ResolveImports(func(location ast.Location) (program *ast.Program, err error) {
		switch location := location.(type) {
		case ast.StringLocation:
			filename := string(location)
			imported, _, code, err := parser.ParseProgramFromFile(filename)
			codes[filename] = code
			must(err, filename)
			return imported, nil

		default:
			return nil, fmt.Errorf("cannot import `%s`. only files are supported", location)
		}
	})
	must(err, filename)

	standardLibraryFunctions := append(stdlib.BuiltinFunctions, stdlib.HelperFunctions...)
	valueDeclarations := standardLibraryFunctions.ToValueDeclarations()
	typeDeclarations := stdlib.BuiltinTypes.ToTypeDeclarations()

	location := runtime.FileLocation(filename)
	checker, err := sema.NewChecker(
		program,
		location,
		sema.WithPredeclaredValues(valueDeclarations),
		sema.WithPredeclaredTypes(typeDeclarations),
	)
	must(err, filename)

	must(checker.Check(), filename)

	values := standardLibraryFunctions.ToValues()

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(values),
	)
	must(err, filename)

	must(inter.Interpret(), filename)

	return inter, checker, func(err error) {
		must(err, filename)
	}
}

func ExitWithError(message string) {
	print(runtime.FormatErrorMessage(message, true))
	os.Exit(1)
}
