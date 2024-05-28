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

	RandomSourceFuncSig = ComputeFunctionSelector("getRandomSource", []string{"uint64"})

	RevertibleRandomFuncSig = ComputeFunctionSelector("revertibleRandom", nil)

	// FlowBlockHeightFixedGas is set to match the `number` opCode (0x43)
	FlowBlockHeightFixedGas = uint64(2)
	// ProofVerifierBaseGas covers the cost of decoding, checking capability the resource
	// and the rest of operations excluding signature verification
	ProofVerifierBaseGas = uint64(1_000)
	// ProofVerifierGasMultiplerPerSignature is set to match `ECRECOVER`
	// but we might increase this in the future
	ProofVerifierGasMultiplerPerSignature = uint64(3_000)

	// RandomSourceGas covers the cost of obtaining random sournce bytes
	RandomSourceGas = uint64(1_000)

	// RevertibleRandomGas covers the cost of calculating a revertible random bytes
	RevertibleRandomGas = uint64(1_000)

	// errUnexpectedInput is returned when the function that doesn't expect an input
	// argument, receives one
	errUnexpectedInput = fmt.Errorf("unexpected input is provided")
)

// ArchContract return a procompile for the Cadence Arch contract
// which facilitates access of Flow EVM environment into the Cadence environment.
// for more details see this Flip 223.
func ArchContract(
	address types.Address,
	heightProvider func() (uint64, error),
	proofVer func(*types.COAOwnershipProofInContext) (bool, error),
	randomSourceProvider func(uint64) (uint64, error),
	revertibleRandomGenerator func() (uint64, error),
) types.Precompile {
	return MultiFunctionPrecompileContract(
		address,
		[]Function{
			&flowBlockHeight{heightProvider},
			&proofVerifier{proofVer},
			&randomnessSource{randomSourceProvider},
			&revertibleRandom{revertibleRandomGenerator},
		},
	)
}

type flowBlockHeight struct {
	flowBlockHeightLookUp func() (uint64, error)
}

var _ Function = &flowBlockHeight{}

func (c *flowBlockHeight) FunctionSelector() FunctionSelector {
	return FlowBlockHeightFuncSig
}

func (c *flowBlockHeight) ComputeGas(input []byte) uint64 {
	return FlowBlockHeightFixedGas
}

func (c *flowBlockHeight) Run(input []byte) ([]byte, error) {
	if len(input) > 0 {
		return nil, errUnexpectedInput
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

type proofVerifier struct {
	proofVerifier func(*types.COAOwnershipProofInContext) (bool, error)
}

var _ Function = &proofVerifier{}

func (f *proofVerifier) FunctionSelector() FunctionSelector {
	return ProofVerifierFuncSig
}

func (f *proofVerifier) ComputeGas(input []byte) uint64 {
	// we compute the gas using a fixed base fee and extra fees
	// per signatures. Note that the input data is already trimmed from the function selector
	// and the remaining is ABI encoded of the inputs

	// skip to the encoded signature part of args (skip address and bytes32 data part)
	index := EncodedAddressSize + Bytes32DataReadSize
	// Reading the encoded signature bytes
	encodedSignature, err := ReadBytes(input, index)
	if err != nil {
		// if any error run would anyway fail, so returning any non-zero value here is fine
		return ProofVerifierBaseGas
	}
	// this method would return the number of signatures from the encoded signature data
	// this saves the extra time needed for full decoding
	// given ComputeGas function is called before charging the gas, we need to keep
	// this function as light as possible
	count, err := types.COAOwnershipProofSignatureCountFromEncoded(encodedSignature)
	if err != nil {
		// if any error run would anyway fail, so returning any non-zero value here is fine
		return ProofVerifierBaseGas
	}
	return ProofVerifierBaseGas + uint64(count)*ProofVerifierGasMultiplerPerSignature
}

func (f *proofVerifier) Run(input []byte) ([]byte, error) {
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

var _ Function = &randomnessSource{}

type randomnessSource struct {
	randomSourceProvider func(uint64) (uint64, error)
}

func (r *randomnessSource) FunctionSelector() FunctionSelector {
	return RandomSourceFuncSig
}

func (r *randomnessSource) ComputeGas(input []byte) uint64 {
	return RandomSourceGas
}

func (r *randomnessSource) Run(input []byte) ([]byte, error) {
	height, err := ReadUint64(input, 0)
	if err != nil {
		return nil, err
	}
	rand, err := r.randomSourceProvider(height)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, EncodedUint64Size)
	err = EncodeUint64(rand, buf, 0)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

var _ Function = &revertibleRandom{}

type revertibleRandom struct {
	revertibleRandomGenerator func() (uint64, error)
}

func (r *revertibleRandom) FunctionSelector() FunctionSelector {
	return RevertibleRandomFuncSig
}

func (r *revertibleRandom) ComputeGas(input []byte) uint64 {
	return RevertibleRandomGas
}

func (r *revertibleRandom) Run(input []byte) ([]byte, error) {
	rand, err := r.revertibleRandomGenerator()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, EncodedUint64Size)
	err = EncodeUint64(rand, buf, 0)
	if err != nil {
		return nil, err
	}

	return buf, nil
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

	encodedProof, err := ReadBytes(input, index)
	if err != nil {
		return nil, err
	}

	return types.NewCOAOwnershipProofInContext(
		hash,
		types.Address(caller),
		encodedProof,
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
