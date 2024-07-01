package handler

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func preparePrecompiledContracts(
	evmContractAddress flow.Address,
	randomBeaconAddress flow.Address,
	addressAllocator types.AddressAllocator,
	backend types.Backend,
) []types.PrecompiledContract {
	archAddress := addressAllocator.AllocatePrecompileAddress(1)
	archContract := precompiles.ArchContract(
		archAddress,
		blockHeightProvider(backend),
		coaOwnershipProofValidator(evmContractAddress, backend),
		randomSourceProvider(randomBeaconAddress, backend),
		revertibleRandomGenerator(backend),
	)
	return []types.PrecompiledContract{archContract}
}

func blockHeightProvider(backend types.Backend) func() (uint64, error) {
	return func() (uint64, error) {
		h, err := backend.GetCurrentBlockHeight()
		if types.IsAFatalError(err) || types.IsABackendError(err) {
			panic(err)
		}
		return h, err
	}
}

const RandomSourceTypeValueFieldName = "value"

func randomSourceProvider(contractAddress flow.Address, backend types.Backend) func(uint64) ([]byte, error) {
	return func(blockHeight uint64) ([]byte, error) {
		value, err := backend.Invoke(
			environment.ContractFunctionSpec{
				AddressFromChain: func(_ flow.Chain) flow.Address {
					return contractAddress
				},
				LocationName: "RandomBeaconHistory",
				FunctionName: "sourceOfRandomness",
				ArgumentTypes: []sema.Type{
					sema.UInt64Type,
				},
			},
			[]cadence.Value{
				cadence.NewUInt64(blockHeight),
			},
		)
		if err != nil {
			if types.IsAFatalError(err) || types.IsABackendError(err) {
				panic(err)
			}
			return nil, err
		}

		data, ok := value.(cadence.Struct)
		if !ok {
			return nil, fmt.Errorf("invalid output data received from getRandomSource")
		}

		cadenceArray := cadence.SearchFieldByName(data, RandomSourceTypeValueFieldName).(cadence.Array)
		source := make([]byte, 32)
		for i := range source {
			source[i] = byte(cadenceArray.Values[i].(cadence.UInt8))
		}

		return source, nil
	}
}

func revertibleRandomGenerator(backend types.Backend) func() (uint64, error) {
	return func() (uint64, error) {
		rand := make([]byte, 8)
		err := backend.ReadRandom(rand)
		if err != nil {
			return 0, err
		}

		return binary.BigEndian.Uint64(rand), nil
	}
}

const ValidationResultTypeIsValidFieldName = "isValid"

func coaOwnershipProofValidator(contractAddress flow.Address, backend types.Backend) func(proof *types.COAOwnershipProofInContext) (bool, error) {
	return func(proof *types.COAOwnershipProofInContext) (bool, error) {
		value, err := backend.Invoke(
			environment.ContractFunctionSpec{
				AddressFromChain: func(_ flow.Chain) flow.Address {
					return contractAddress
				},
				LocationName: "EVM",
				FunctionName: "validateCOAOwnershipProof",
				ArgumentTypes: []sema.Type{
					types.FlowAddressSemaType,
					types.PublicPathSemaType,
					types.SignedDataSemaType,
					types.KeyIndicesSemaType,
					types.SignaturesSemaType,
					types.AddressBytesSemaType,
				},
			},
			proof.ToCadenceValues(),
		)
		if err != nil {
			if types.IsAFatalError(err) || types.IsABackendError(err) {
				panic(err)
			}
			return false, err
		}
		data, ok := value.(cadence.Struct)
		if !ok {
			return false, fmt.Errorf("invalid output data received from validateCOAOwnershipProof")
		}

		isValidValue := cadence.SearchFieldByName(data, ValidationResultTypeIsValidFieldName)
		if isValidValue == nil {
			return false, fmt.Errorf("invalid output data received from validateCOAOwnershipProof")
		}

		return bool(isValidValue.(cadence.Bool)), nil
	}
}
