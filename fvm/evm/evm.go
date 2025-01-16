package evm

import (
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/backends"
	evm "github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/impl"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func ContractAccountAddress(chainID flow.ChainID) flow.Address {
	return systemcontracts.SystemContractsForChain(chainID).EVMContract.Address
}

func StorageAccountAddress(chainID flow.ChainID) flow.Address {
	return systemcontracts.SystemContractsForChain(chainID).EVMStorage.Address
}

func SetupEnvironment(
	chainID flow.ChainID,
	fvmEnv environment.Environment,
	runtimeEnv runtime.Environment,
) error {
	sc := systemcontracts.SystemContractsForChain(chainID)
	randomBeaconAddress := sc.RandomBeaconHistory.Address
	flowTokenAddress := sc.FlowToken.Address

	backend := backends.NewWrappedEnvironment(fvmEnv)
	emulator := evm.NewEmulator(backend, StorageAccountAddress(chainID))
	blockStore := handler.NewBlockStore(chainID, backend, StorageAccountAddress(chainID))
	addressAllocator := handler.NewAddressAllocator()

	evmContractAddress := ContractAccountAddress(chainID)

	contractHandler := handler.NewContractHandler(
		chainID,
		evmContractAddress,
		common.Address(flowTokenAddress),
		randomBeaconAddress,
		blockStore,
		addressAllocator,
		backend,
		emulator,
	)

	internalEVMContractValue := impl.NewInternalEVMContractValue(
		nil,
		contractHandler,
		evmContractAddress,
	)

	stdlib.SetupEnvironment(
		runtimeEnv,
		internalEVMContractValue,
		evmContractAddress,
	)

	return nil
}
