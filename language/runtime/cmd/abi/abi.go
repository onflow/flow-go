package abi

import (
	"encoding/json"
	"os"

	"github.com/dapperlabs/flow-go/language/runtime/cmd"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/types/encoding"
)

/*
 Generates ABIs from provided Cadence file
*/
func GenerateABI(args []string) {

	if len(args) < 1 {
		cmd.ExitWithError("no input file")
	}

	jsonData := GetABIForFile(args[0])

	_, err := os.Stdout.Write(jsonData)

	if err != nil {
		panic(err)
	}

}

func GetABIForFile(filename string) []byte {

	_, checker, _ := cmd.PrepareInterpreter(filename)

	exportedTypes := map[string]types.Type{}

	values := checker.UserDefinedValues()
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

	return jsonData

}
