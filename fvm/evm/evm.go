package evm

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	evm "github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func RootAccountAddress(chainID flow.ChainID) (flow.Address, error) {
	// TODO handle other chains
	switch chainID {
	case flow.Emulator:
		return chainID.Chain().AddressAtIndex(environment.EVMAccountIndex)
	default:
		return flow.Address{}, fmt.Errorf("unsupported chain %s", chainID)
	}
}

func SetupEnvironment(
	chainID flow.ChainID,
	backend types.Backend,
	env runtime.Environment,
	service flow.Address,
	flowToken flow.Address,
) error {
	// TODO: setup proper root address based on chainID
	evmRootAddress, err := RootAccountAddress(chainID)
	if err != nil {
		return err
	}

	db, err := database.NewDatabase(backend, evmRootAddress)
	if err != nil {
		return err
	}

	em := evm.NewEmulator(db)

	bs, err := handler.NewBlockStore(backend, evmRootAddress)
	if err != nil {
		return err
	}

	contractHandler := handler.NewContractHandler(common.Address(flowToken), bs, backend, em)

	stdlib.SetupEnvironment(env, contractHandler, service)

	return nil
}
