package main

import (
	"github.com/dapperlabs/flow-go/sdk/abi/generation/code"
	types2 "github.com/dapperlabs/flow-go/sdk/abi/types"
)

func main() {

	//TODO fix for final push
	//if len(os.Args) != 3 {
	//	panic("use input_file output_file")
	//}

	//abiFilename := os.Args[1]

	types := map[string]*types2.Composite{
		"Car": {
			Fields: map[string]*types2.Field{
				"fullName": {
					Identifier: "fullName",
					Type:       types2.String{},
				},
			},
			Identifier: "Car",
			Initializers: [][]*types2.Parameter{
				{
					&types2.Parameter{
						Field: types2.Field{
							Identifier: "model",
							Type:       types2.String{},
						},
						Label: "",
					},
					&types2.Parameter{
						Field: types2.Field{
							Identifier: "make",
							Type:       types2.String{},
						},
						Label: "",
					},
				},
			},
		},
	}

	code.GenerateGo("example", types)

}
