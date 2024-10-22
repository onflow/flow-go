package testutils

import (
	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/fvm/evm/debug"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func SetupHandler(
	chainID flow.ChainID,
	backend types.Backend,
	rootAddr flow.Address,
) *handler.ContractHandler {
	return handler.NewContractHandler(
		chainID,
		rootAddr,
		common.MustBytesToAddress(systemcontracts.SystemContractsForChain(chainID).FlowToken.Address.Bytes()),
		rootAddr,
		handler.NewBlockStore(chainID, backend, rootAddr),
		handler.NewAddressAllocator(),
		backend,
		emulator.NewEmulator(backend, rootAddr),
		debug.NopTracer,
	)
}

type TestPrecompiledContract struct {
	RequiredGasFunc func(input []byte) uint64
	RunFunc         func(input []byte) ([]byte, error)
	AddressFunc     func() types.Address
}

var _ types.PrecompiledContract = &TestPrecompiledContract{}

// RequiredGas returns the contract gas use
func (pc *TestPrecompiledContract) RequiredGas(input []byte) uint64 {
	if pc.RequiredGasFunc == nil {
		panic("RequiredGasFunc is not set for the test precompiled contract")
	}
	return pc.RequiredGasFunc(input)
}

// Run runs the precompiled contract
func (pc *TestPrecompiledContract) Run(input []byte) ([]byte, error) {
	if pc.RunFunc == nil {
		panic("RunFunc is not set for the test precompiled contract")
	}
	return pc.RunFunc(input)
}

// Address returns the address that this contract is deployed to
func (pc *TestPrecompiledContract) Address() types.Address {
	if pc.AddressFunc == nil {
		panic("AddressFunc is not set for the test precompiled contract")
	}
	return pc.AddressFunc()
}
