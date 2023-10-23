package evm

import (
	"github.com/onflow/cadence/runtime"

	evm "github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/stdlib/emulator"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func SetupEnvironment(
	chainID flow.ChainID,
	backend types.Backend,
	env runtime.Environment,
) error {
	// TODO: setup proper root address based on chainID
	evmRootAddress, err := emulator.EVMRootAccountAddress(chainID)
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
	handler := handler.NewContractHandler(bs, backend, em)
	// TODO: pass proper Flex type definition based on environment
	flexTypeDefinition := emulator.FlexTypeDefinition
	env.DeclareValue(stdlib.NewFlexStandardLibraryValue(nil, flexTypeDefinition, handler))
	env.DeclareType(stdlib.NewFlexStandardLibraryType(flexTypeDefinition))
	return nil
}
