package abi

import (
	"encoding/json"
	"os"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/language/runtime/cmd"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/types/encoding"
)

/*
 Generates ABIs from provided Cadence file
*/
func GenerateAbi(args []string) {

	_, checker, _ := cmd.PrepareInterpreter(args)

	exportedTypes := map[string]types.Type{}

	values := checker.GlobalNewValues()
	for _, variable := range values {
		exportable, ok := variable.Type.(sema.ExportableType)

		if ok {
			exportedType := exportable.Export(checker.Program, variable)
			exportedTypes[variable.Identifier] = exportedType
		}
	}

	encoder := encoding.NewEncoder()

	for name, type_ := range exportedTypes {
		encoder.Encode(name, type_)
	}

	jsonData, err := json.MarshalIndent(encoder.Get(), "", "  ")

	if err != nil {
		panic(err)
	}
	os.Stdout.Write(jsonData)

	//fmt.Printf("%+v\n", exportedTypes)
}

//func PrettyPrintError(err error, filename string, codes map[string]string) {
//	i := 0
//	printErr := func(err error, filename string) {
//		if i > 0 {
//			println()
//		}
//		print(runtime.PrettyPrintError(err, filename, codes[filename], true))
//		i += 1
//	}
//
//	if parserError, ok := err.(parser.Error); ok {
//		for _, err := range parserError.Errors {
//			printErr(err, filename)
//		}
//	} else if checkerError, ok := err.(*sema.CheckerError); ok {
//		for _, err := range checkerError.Errors {
//			printErr(err, filename)
//			if err, ok := err.(*sema.ImportedProgramError); ok {
//				filename := string(err.ImportLocation.(ast.StringLocation))
//				for _, err := range err.CheckerError.Errors {
//					PrettyPrintError(err, filename, codes)
//				}
//			}
//		}
//	} else if locatedErr, ok := err.(ast.HasImportLocation); ok {
//		location := locatedErr.ImportLocation()
//		if location != nil {
//			filename = string(location.(ast.StringLocation))
//		}
//		printErr(err, filename)
//	} else {
//		printErr(err, filename)
//	}
//}

func exitWithError(message string) {
	print(runtime.FormatErrorMessage(message, true))
	os.Exit(1)
}
