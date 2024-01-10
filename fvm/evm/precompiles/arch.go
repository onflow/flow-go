package precompiles

import (
	"encoding/binary"
	"fmt"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/onflow/flow-go/fvm/evm/types"
)

var (
	FlowBlockHeightMethodID = [4]byte{1, 2, 3, 4} // TODO fill in with proper value
	FlowBlockHeightFixedGas = uint64(0)           // TODO update me with a proper value

	VerifyProofMethodID = [4]byte{3, 4, 5, 6} // TODO fill in with proper value
	VerifyProofFixedGas = uint64(0)           // TODO update me with a proper value

	true32Byte  = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	false32Byte = make([]byte, 32)
)

type flowBlockHeightCallable struct {
	flowBlockHeightLookUp func() (uint64, error)
}

func (c *flowBlockHeightCallable) MethodID() MethodID {
	return FlowBlockHeightMethodID
}

func (c *flowBlockHeightCallable) ComputeGas(input []byte) uint64 {
	return FlowBlockHeightFixedGas
}

func (c *flowBlockHeightCallable) Run(input []byte) ([]byte, error) {
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

type verifyProofCallable struct {
	verifyProof func([]byte) (bool, error)
}

func (c *verifyProofCallable) MethodID() MethodID {
	return VerifyProofMethodID
}

func (c *verifyProofCallable) ComputeGas(input []byte) uint64 {
	return VerifyProofFixedGas
}

func (c *verifyProofCallable) Run(input []byte) ([]byte, error) {
	res, err := c.verifyProof(input)
	if err != nil {
		return nil, err
	}
	if res {
		return true32Byte, err
	}
	return false32Byte, err
}

func ArchContract(
	address types.Address,
	heightProvider func() (uint64, error),
	verifyProof func([]byte) (bool, error),
) types.Precompile {
	fbh := &flowBlockHeightCallable{heightProvider}
	vc := &verifyProofCallable{verifyProof}
	return multiMethodPrecompileContract(
		address,
		map[MethodID]Callable{
			fbh.MethodID(): fbh,
			vc.MethodID():  vc,
		},
	)
}
