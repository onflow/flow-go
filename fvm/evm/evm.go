package evm

import (
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/backends"
	evm "github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func SetupEnvironment(
	chainID flow.ChainID,
	fvmEnv environment.Environment,
	runtimeEnv runtime.Environment,
) error {
	sc := systemcontracts.SystemContractsForChain(chainID)
	randomBeaconAddress := sc.RandomBeaconHistory.Address
	flowTokenAddress := sc.FlowToken.Address
	storageAddress := sc.EVMStorage.Address
	evmAddress := sc.EVMContract.Address

	backend := backends.NewWrappedEnvironment(fvmEnv)
	emulator := evm.NewEmulator(backend, storageAddress)
	blockStore := handler.NewBlockStore(backend, storageAddress)
	addressAllocator := handler.NewAddressAllocator()

	contractHandler := handler.NewContractHandler(
		chainID,
		evmAddress,
		common.Address(flowTokenAddress),
		randomBeaconAddress,
		blockStore,
		addressAllocator,
		backend,
		emulator,
	)

	stdlib.SetupEnvironment(
		runtimeEnv,
		contractHandler,
		evmAddress,
	)

	return nil
}
