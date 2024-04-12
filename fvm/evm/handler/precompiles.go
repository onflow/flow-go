package handler

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func preparePrecompiles(
	evmContractAddress flow.Address,
	addressAllocator types.AddressAllocator,
	backend types.Backend,
) []types.Precompile {
	archAddress := addressAllocator.AllocatePrecompileAddress(1)
	archContract := precompiles.ArchContract(
		archAddress,
		blockHeightProvider(backend),
		coaOwnershipProofValidator(evmContractAddress, backend),
	)
	return []types.Precompile{archContract}
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
		if !ok || len(data.Fields) == 0 {
			return false, fmt.Errorf("invalid output data received from validateCOAOwnershipProof")
		}
		return bool(data.Fields[0].(cadence.Bool)), nil
	}
}
