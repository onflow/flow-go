package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/dapperlabs/flow-go/sdk/abi/encoding/types"
	"github.com/dapperlabs/flow-go/sdk/abi/generation/code"
	abiTypes "github.com/dapperlabs/flow-go/sdk/abi/types"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {

	if len(os.Args) != 4 {
		println("use package_name input_file output_file")
		os.Exit(1)
	}

	pkg := os.Args[1]
	inputFile := os.Args[2]
	outputFile := os.Args[3]

	data, err := ioutil.ReadFile(inputFile)
	check(err)

	allTypes, err := types.Decode(data)

	compositeTypes := map[string]*abiTypes.Composite{}

	for name, typ := range allTypes {

		switch composite := typ.(type) {
		case *abiTypes.Resource:
			compositeTypes[name] = &composite.Composite
		case *abiTypes.Struct:
			compositeTypes[name] = &composite.Composite
		default:
			_, err := fmt.Fprintf(os.Stderr, "Definition %s of type %T is not supported, skipping\n", name, typ)
			check(err)
		}

		if composite, ok := typ.(*abiTypes.Composite); ok {
			compositeTypes[name] = composite
		} else {

		}
	}

	file, err := os.Create(outputFile)
	defer file.Close()

	check(err)

	err = code.GenerateGo(pkg, compositeTypes, file)
	check(err)
}
