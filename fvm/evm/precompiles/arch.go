package precompiles

import (
	"encoding/binary"
	"fmt"

	gethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/evm/types"
)

var (
	FlowBlockHeightFuncSig = ComputeFunctionSignature("flowBlockHeight", nil)
	// TODO update me with a higher value if needed
	FlowBlockHeightFixedGas = uint64(1)
)

// ArchContract return a procompile for the Cadence Arch contract
func ArchContract(
	address types.Address,
	heightProvider func() (uint64, error),
) types.Precompile {
	return MultiFunctionPrecompileContract(
		address,
		[]Function{&flowBlockHeightFunction{heightProvider}},
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
	// the EVM works natively in 256-bit words,
	// we left pad to that size to prevent extra gas consumtion for masking.
	return gethCommon.LeftPadBytes(encoded, 32), nil
}
