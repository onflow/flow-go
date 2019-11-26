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
func GenerateABI(args []string, pretty bool) {

	if len(args) < 1 {
		cmd.ExitWithError("no input file")
	}

	jsonData := GetABIForFile(args[0], pretty)

	_, err := os.Stdout.Write(jsonData)

	if err != nil {
		panic(err)
	}

}

func GetABIForFile(filename string, pretty bool) []byte {

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

	marshall := func() ([]byte, error) {
		if pretty {
			return json.MarshalIndent(encoder.Get(), "", "  ")
		} else {
			return json.Marshal(encoder.Get())
		}
	}

	jsonData, err := marshall()

	if err != nil {
		panic(err)
	}

	return jsonData
}
