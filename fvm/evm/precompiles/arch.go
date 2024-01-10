package precompiles

import (
	"encoding/binary"
	"fmt"

	gethVM "github.com/ethereum/go-ethereum/core/vm"
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
	flowBlockHeight uint64
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
	// TODO maybe replace this with what abi encode does
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, c.flowBlockHeight)
	return encoded, nil
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

func GetArchContract(flowBlockHeight uint64, verifyProof func([]byte) (bool, error)) gethVM.PrecompiledContract {
	fbh := &flowBlockHeightCallable{flowBlockHeight}
	vc := &verifyProofCallable{verifyProof}
	return GetPrecompileContract(
		map[MethodID]Callable{
			fbh.MethodID(): fbh,
			vc.MethodID():  vc,
		},
	)
}
