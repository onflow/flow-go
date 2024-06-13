package migrations

import (
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
)

// NewAtreeRegisterMigratorRuntime returns a new runtime to be used with the AtreeRegisterMigrator.
func NewAtreeRegisterMigratorRuntime(
	address common.Address,
	regs registers.Registers,
) (
	*AtreeRegisterMigratorRuntime,
	error,
) {
	transactionState := state.NewTransactionState(
		registers.StorageSnapshot{
			Registers: regs,
		},
		state.DefaultParameters(),
	)
	accounts := environment.NewAccounts(transactionState)

	accountsAtreeLedger := util.NewAccountsAtreeLedger(accounts)
	storage := runtime.NewStorage(accountsAtreeLedger, nil)

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		&interpreter.Config{
			Storage: storage,
		},
	)
	if err != nil {
		return nil, err
	}

	return &AtreeRegisterMigratorRuntime{
		Address:             address,
		TransactionState:    transactionState,
		Interpreter:         inter,
		Storage:             storage,
		AccountsAtreeLedger: accountsAtreeLedger,
	}, nil
}

type AtreeRegisterMigratorRuntime struct {
	TransactionState    state.NestedTransactionPreparer
	Interpreter         *interpreter.Interpreter
	Storage             *runtime.Storage
	Address             common.Address
	AccountsAtreeLedger *util.AccountsAtreeLedger
}
