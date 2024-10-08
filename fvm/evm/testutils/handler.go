package testutils

import (
	"github.com/onflow/cadence/runtime/common"

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
