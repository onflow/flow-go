/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"example.com/cadence-initial/runtime"
	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/interpreter"
	"example.com/cadence-initial/runtime/parser"
	"example.com/cadence-initial/runtime/sema"
	"example.com/cadence-initial/runtime/stdlib"
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

func PrepareCheckerFromFile(filename string) (*sema.Checker, func(error)) {
	codeBytes, err := ioutil.ReadFile(filename)

	checker, must := PrepareChecker(string(codeBytes), filename)
	must(err)

	return checker, must
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
	must(err)

	values := standardLibraryFunctions().ToValues()

	var uuid uint64

	inter, err := interpreter.NewInterpreter(
		checker,
		interpreter.WithPredefinedValues(values),
		interpreter.WithUUIDHandler(func() uint64 {
			defer func() { uuid++ }()
			return uuid
		}),
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
