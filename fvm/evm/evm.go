package evm

import (
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	evm "github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/types"
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
	backend types.Backend,
	env runtime.Environment,
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

	em := evm.NewEmulator(backend, evmStorageAccountAddress)

	bs, err := handler.NewBlockStore(backend, evmStorageAccountAddress)
	if err != nil {
		return err
	}

	aa, err := handler.NewAddressAllocator(backend, evmStorageAccountAddress)
	if err != nil {
		return err
	}

	contractHandler := handler.NewContractHandler(evmContractAccountAddress, common.Address(flowToken), bs, aa, backend, em)

	stdlib.SetupEnvironment(env, contractHandler, evmContractAccountAddress)

	return nil
}
