package code

import (
	"fmt"
	"os"
	"testing"

	"github.com/dave/jennifer/jen"

	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

func TestFoo(t *testing.T) {
	f := jen.NewFile("main")

	statements := make([]jen.Code, 0)

	//statements = append(statements, converterFor(&types.Int{}))

	statements = append(statements, converterFor(&types.Optional{Of: &types.Optional{Of: &types.Int{}}}))

	statements = append(statements, converterFor(&types.Optional{Of: &types.Int{}}))

	converterFor(&types.VariableSizedArray{
		ElementType: &types.String{},
	})

	converterFor(&types.VariableSizedArray{
		ElementType: &types.Optional{
			Of: &types.String{},
		},
	})

	for _, fun := range converterFunctions {
		f.Add(fun)
	}

	//f.Block(statements...)

	file, _ := os.Create("/Users/makspawlak/go/src/github.com/dapperlabs/flow-go/sdk/examples/abi/generated/maksio.go")
	f.Render(file)

	fmt.Printf("%#v", f)

}
