package precompiles

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/evm/types"
)

var (
	FlowBlockHeightFuncSig = ComputeFunctionSelector("flowBlockHeight", nil)
	// TODO: fix me
	ProofVerifierFuncSig = ComputeFunctionSelector(
		"verifyCOAOwnershipProof",
		[]string{"address", "bytes32", "bytes"},
	)

	// FlowBlockHeightFixedGas is set to match the `number` opCode (0x43)
	FlowBlockHeightFixedGas = uint64(2)
	// ProofVerifierBaseGas covers the cost of decoding, checking capability the resource
	// and the rest of operations excluding signature verification
	ProofVerifierBaseGas = uint64(1_000)
	// ProofVerifierGasMultiplerPerSignature is set to match `ECRECOVER`
	// but we might increase this in the future
	ProofVerifierGasMultiplerPerSignature = uint64(3_000)
)

// ArchContract return a procompile for the Cadence Arch contract
// which facilitates access of Flow EVM environment into the Cadence environment.
// for more details see this Flip 223.
func ArchContract(
	address types.Address,
	heightProvider func() (uint64, error),
	proofVerifier func(*types.COAOwnershipProofInContext) (bool, error),
) types.Precompile {
	return MultiFunctionPrecompileContract(
		address,
		[]Function{
			&flowBlockHeightFunction{heightProvider},
			&proofVerifierFunction{proofVerifier},
		},
	)
}

type flowBlockHeightFunction struct {
	flowBlockHeightLookUp func() (uint64, error)
}

func (c *flowBlockHeightFunction) FunctionSelector() FunctionSelector {
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
	// EVM works natively in 256-bit words,
	// Encode to 256-bit is the common practice to prevent extra gas consumtion for masking.
	buffer := make([]byte, EncodedUint64Size)
	return buffer, EncodeUint64(bh, buffer, 0)
}

type proofVerifierFunction struct {
	proofVerifier func(*types.COAOwnershipProofInContext) (bool, error)
}

func (f *proofVerifierFunction) FunctionSelector() FunctionSelector {
	return ProofVerifierFuncSig
}

func (f *proofVerifierFunction) ComputeGas(input []byte) uint64 {
	// skip first two parts
	index := FixedSizeUnitDataReadSize + Bytes32DataReadSize
	encodedSignature, err := ReadBytes(input, index)
	if err != nil {
		// if any error run would anyway fail, so returning any non-zero value here is fine
		return ProofVerifierBaseGas
	}
	count, err := types.COAOwnershipProofSignatureCountFromEncoded(encodedSignature)
	if err != nil {
		// if any error run would anyway fail, so returning any non-zero value here is fine
		return ProofVerifierBaseGas
	}
	return ProofVerifierBaseGas + uint64(count)*ProofVerifierGasMultiplerPerSignature
}

func (f *proofVerifierFunction) Run(input []byte) ([]byte, error) {
	proof, err := DecodeABIEncodedProof(input)
	if err != nil {
		return nil, err
	}
	verified, err := f.proofVerifier(proof)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, EncodedBoolSize)
	return buffer, EncodeBool(verified, buffer, 0)
}

func DecodeABIEncodedProof(input []byte) (*types.COAOwnershipProofInContext, error) {
	index := 0
	caller, err := ReadAddress(input, index)
	index += FixedSizeUnitDataReadSize
	if err != nil {
		return nil, err
	}

	hash, err := ReadBytes32(input, index)
	index += Bytes32DataReadSize
	if err != nil {
		return nil, err
	}

	signature, err := ReadBytes(input, index)
	if err != nil {
		return nil, err
	}

	return types.NewCOAOwnershipProofInContext(
		hash,
		types.Address(caller),
		signature,
	)
}

func ABIEncodeProof(proof *types.COAOwnershipProofInContext) ([]byte, error) {
	encodedProof, err := proof.COAOwnershipProof.Encode()
	if err != nil {
		return nil, err
	}
	bufferSize := EncodedAddressSize +
		EncodedBytes32Size +
		SizeNeededForBytesEncoding(encodedProof)

	abiEncodedData := make([]byte, bufferSize)
	index := 0
	err = EncodeAddress(proof.EVMAddress.ToCommon(), abiEncodedData, index)
	if err != nil {
		return nil, err
	}
	index += EncodedAddressSize
	err = EncodeBytes32(proof.SignedData, abiEncodedData, index)
	if err != nil {
		return nil, err
	}
	index += EncodedBytes32Size
	err = EncodeBytes(encodedProof, abiEncodedData, index, index+EncodedUint64Size)
	if err != nil {
		return nil, err
	}
	return abiEncodedData, nil
}
