package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	migrationSnapshot "github.com/onflow/flow-go/cmd/util/ledger/util/snapshot"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
)

// NewAtreeRegisterMigratorRuntime returns a new runtime to be used with the AtreeRegisterMigrator.
func NewAtreeRegisterMigratorRuntime(
	address common.Address,
	payloads []*ledger.Payload,
) (
	*AtreeRegisterMigratorRuntime,
	error,
) {
	snapshot, err := migrationSnapshot.NewPayloadSnapshot(payloads, migrationSnapshot.LargeChangeSetOrReadonlySnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload snapshot: %w", err)
	}
	transactionState := state.NewTransactionState(snapshot, state.DefaultParameters())
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
		Payloads:            payloads,
		Snapshot:            snapshot,
		TransactionState:    transactionState,
		Interpreter:         inter,
		Storage:             storage,
		AccountsAtreeLedger: accountsAtreeLedger,
	}, nil
}

type AtreeRegisterMigratorRuntime struct {
	Snapshot            migrationSnapshot.MigrationSnapshot
	TransactionState    state.NestedTransactionPreparer
	Interpreter         *interpreter.Interpreter
	Storage             *runtime.Storage
	Payloads            []*ledger.Payload
	Address             common.Address
	AccountsAtreeLedger *util.AccountsAtreeLedger
}
