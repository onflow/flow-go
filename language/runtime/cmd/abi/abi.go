package abi

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/dapperlabs/flow-go/language/runtime/cmd"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	typesEncoding "github.com/dapperlabs/flow-go/sdk/abi/encoding/types"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

// GenerateABI generates ABIs from provided Cadence file
func GenerateABI(args []string, pretty bool) error {

	if len(args) < 1 {
		return errors.New("no input file given")
	}

	jsonData := GetABIJSONFromCadenceFile(args[0], pretty)

	_, err := os.Stdout.Write(jsonData)

	return err
}

func exportTypesFromChecker(checker *sema.Checker) map[string]types.Type {
	exportedTypes := map[string]types.Type{}

	values := checker.UserDefinedValues()
	for _, variable := range values {
		exportable, ok := variable.Type.(sema.ExportableType)

		if ok {
			exportedType := exportable.Export(checker.Program, variable)
			exportedTypes[variable.Identifier] = exportedType
		}
	}

	return exportedTypes
}

func encodeTypesAsJson(types map[string]types.Type, pretty bool) ([]byte, error) {
	encoder := typesEncoding.NewEncoder()

	for name, typ := range types {
		encoder.Encode(name, typ)
	}

	if pretty {
		return json.MarshalIndent(encoder.Get(), "", "  ")
	}
	return json.Marshal(encoder.Get())
}

func GetABIJSONFromCadenceCode(code string, pretty bool, filename string) []byte {
	checker, _ := cmd.PrepareChecker(code, filename)

	exportedTypes := exportTypesFromChecker(checker)

	jsonData, err := encodeTypesAsJson(exportedTypes, pretty)

	if err != nil {
		panic(err)
	}

	return jsonData
}

func GetABIJSONFromCadenceFile(filename string, pretty bool) []byte {

	checker, _ := cmd.PrepareCheckerFromFile(filename)

	exportedTypes := exportTypesFromChecker(checker)

	jsonData, err := encodeTypesAsJson(exportedTypes, pretty)

	if err != nil {
		panic(err)
	}

	return jsonData
}

func GetTypesFromCadenceFile(filename string) map[string]types.Type {

	checker, _ := cmd.PrepareCheckerFromFile(filename)

	exportedTypes := exportTypesFromChecker(checker)

	return exportedTypes
}

func GetTypesFromCadenceCode(code string, filename string) map[string]types.Type {

	checker, _ := cmd.PrepareChecker(code, filename)

	exportedTypes := exportTypesFromChecker(checker)

	return exportedTypes
}

func GetTypesFromABIJSONBytes(bytes []byte) (map[string]types.Type, error) {
	return typesEncoding.Decode(bytes)
}
