package abi

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/dapperlabs/flow-go/language/runtime/cmd"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/sdk/abi/encoding/types"
	types2 "github.com/dapperlabs/flow-go/sdk/abi/types"
)

// GenerateABI generates ABIs from provided Cadence file
func GenerateABI(args []string, pretty bool) error {

	if len(args) < 1 {
		return errors.New("no input file given")
	}

	jsonData := GetABIForFile(args[0], pretty)

	_, err := os.Stdout.Write(jsonData)

	return err
}

func GetABIForFile(filename string, pretty bool) []byte {

	_, checker, _ := cmd.PrepareInterpreter(filename)

	exportedTypes := map[string]types2.Type{}

	values := checker.UserDefinedValues()
	for _, variable := range values {
		exportable, ok := variable.Type.(sema.ExportableType)

		if ok {
			exportedType := exportable.Export(checker.Program, variable)
			exportedTypes[variable.Identifier] = exportedType
		}
	}

	encoder := types.NewEncoder()

	for name, typ := range exportedTypes {
		encoder.Encode(name, typ)
	}

	marshal := func() ([]byte, error) {
		if pretty {
			return json.MarshalIndent(encoder.Get(), "", "  ")
		}
		return json.Marshal(encoder.Get())
	}

	jsonData, err := marshal()

	if err != nil {
		panic(err)
	}

	return jsonData
}

func GetABIFromFile(filename string) (map[string]types2.Type, error) {

	return nil, nil
}
