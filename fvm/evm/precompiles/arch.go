package precompiles

import (
	"encoding/binary"
	"fmt"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/onflow/flow-go/fvm/evm/types"
)

var (
	FlowBlockHeightFuncSig  = ComputeFunctionSignature("flowBlockHeight", nil)
	FlowBlockHeightFixedGas = uint64(0) // TODO update me with a proper value
)

func ArchContract(
	address types.Address,
	heightProvider func() (uint64, error),
) types.Precompile {
	fbh := &flowBlockHeightFunction{heightProvider}
	return multiMethodPrecompileContract(
		address,
		map[FunctionSignature]Function{
			fbh.FunctionSignature(): fbh,
		},
	)
}

type flowBlockHeightFunction struct {
	flowBlockHeightLookUp func() (uint64, error)
}

func (c *flowBlockHeightFunction) FunctionSignature() FunctionSignature {
	return FlowBlockHeightFuncSig
}

func (c *flowBlockHeightFunction) ComputeGas(input []byte) uint64 {
	return FlowBlockHeightFixedGas
}

func (c *flowBlockHeightFunction) Run(input []byte) ([]byte, error) {
	if len(input) > 0 {
		return nil, fmt.Errorf("unexpected input is provided")
	}
	bh, err := c.flowBlockHeightLookUp()
	if err != nil {
		return nil, err
	}
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, bh)
	return gethCommon.LeftPadBytes(encoded, 32), nil
}
