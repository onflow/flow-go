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

func SetupEnvironment(
	chainID flow.ChainID,
	fvmEnv environment.Environment,
	runtimeEnv runtime.Environment,
	service flow.Address,
	flowToken flow.Address,
) error {
	evmStorageAccountAddress, err := StorageAccountAddress(chainID)
	if err != nil {
		return err
	}

	evmContractAccountAddress, err := ContractAccountAddress(chainID)
	if err != nil {
		return err
	}

	backend := backends.NewWrappedEnvironment(fvmEnv)

	em := evm.NewEmulator(backend, evmStorageAccountAddress)

	bs := handler.NewBlockStore(backend, evmStorageAccountAddress)

	aa := handler.NewAddressAllocator()

	contractHandler := handler.NewContractHandler(evmContractAccountAddress, common.Address(flowToken), bs, aa, backend, em)

	stdlib.SetupEnvironment(runtimeEnv, contractHandler, evmContractAccountAddress)

	return nil
}
