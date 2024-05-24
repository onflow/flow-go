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

func ContractAccountAddress(chainID flow.ChainID) (flow.Address, error) {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return sc.EVMContract.Address, nil
}

func StorageAccountAddress(chainID flow.ChainID) (flow.Address, error) {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return sc.EVMStorage.Address, nil
}

func RandomBeaconAddress(chainID flow.ChainID) flow.Address {
	return systemcontracts.SystemContractsForChain(chainID).RandomBeaconHistory.Address
}

func SetupEnvironment(
	chainID flow.ChainID,
	fvmEnv environment.Environment,
	runtimeEnv runtime.Environment,
	flowToken flow.Address,
	tracingEnabled bool,
) error {
	evmStorageAccountAddress, err := StorageAccountAddress(chainID)
	if err != nil {
		return err
	}

	evmContractAccountAddress, err := ContractAccountAddress(chainID)
	if err != nil {
		return err
	}

	randomBeaconAddress := RandomBeaconAddress(chainID)

	backend := backends.NewWrappedEnvironment(fvmEnv)

	emulator := evm.NewEmulator(backend, evmStorageAccountAddress)

	blockStore := handler.NewBlockStore(backend, evmStorageAccountAddress)

	addressAllocator := handler.NewAddressAllocator()

	contractHandler := handler.NewContractHandler(
		chainID,
		evmContractAccountAddress,
		common.Address(flowToken),
		randomBeaconAddress,
		blockStore,
		addressAllocator,
		backend,
		emulator,
		tracingEnabled,
	)

	stdlib.SetupEnvironment(
		runtimeEnv,
		contractHandler,
		evmContractAccountAddress,
	)

	return nil
}
