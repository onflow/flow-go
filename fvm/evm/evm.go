package evm

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/backends"
	"github.com/onflow/flow-go/fvm/evm/debug"
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
	tracer debug.EVMTracer,
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

	height, err := backend.GetCurrentBlockHeight()
	if err != nil {
		return err
	}

	block, ok, err := backend.GetBlockAtHeight(height)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("could not retrieve the block at height")
	}

	contractHandler := handler.NewContractHandler(
		chainID,
		flow.Identifier(block.Hash),
		evmContractAccountAddress,
		common.Address(flowToken),
		randomBeaconAddress,
		blockStore,
		addressAllocator,
		backend,
		emulator,
		tracer,
	)

	stdlib.SetupEnvironment(
		runtimeEnv,
		contractHandler,
		evmContractAccountAddress,
	)

	return nil
}
