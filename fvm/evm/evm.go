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
	tracer debug.EVMTracer,
) error {
	sc := systemcontracts.SystemContractsForChain(chainID)
	randomBeaconAddress := sc.RandomBeaconHistory.Address
	flowTokenAddress := sc.FlowToken.Address

	backend := backends.NewWrappedEnvironment(fvmEnv)
	emulator := evm.NewEmulator(backend.Logger(), backend, StorageAccountAddress(chainID))
	blockStore := handler.NewBlockStore(chainID, backend, StorageAccountAddress(chainID))
	addressAllocator := handler.NewAddressAllocator()

	if tracer != debug.NopTracer {
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

		tracer.WithBlockID(flow.Identifier(block.Hash))
	}

	contractHandler := handler.NewContractHandler(
		chainID,
		ContractAccountAddress(chainID),
		common.Address(flowTokenAddress),
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
		ContractAccountAddress(chainID),
	)

	return nil
}
