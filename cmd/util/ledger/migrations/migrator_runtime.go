package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
)

// migratorRuntime is a runtime that can be used to run a migration on a single account
func newMigratorRuntime(
	address common.Address,
	payloads []*ledger.Payload,
) (
	*migratorRuntime,
	error,
) {
	snapshot, err := util.NewPayloadSnapshot(payloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload snapshot: %w", err)
	}
	transactionState := state.NewTransactionState(snapshot, state.DefaultParameters())
	accounts := environment.NewAccounts(transactionState)

	accountsAtreeLedger := util.NewAccountsAtreeLedger(accounts)
	storage := runtime.NewStorage(accountsAtreeLedger, util.NopMemoryGauge{})

	ri := &util.MigrationRuntimeInterface{
		Accounts: accounts,
	}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{
		// Attachments are enabled everywhere except for Mainnet
		AttachmentsEnabled: true,
	})

	env.Configure(
		ri,
		runtime.NewCodesAndPrograms(),
		storage,
		runtime.NewCoverageReport(),
	)

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		env.InterpreterConfig)
	if err != nil {
		return nil, err
	}

	return &migratorRuntime{
		Address:          address,
		Payloads:         payloads,
		Snapshot:         snapshot,
		TransactionState: transactionState,
		Interpreter:      inter,
		Storage:          storage,
		Accounts:         accountsAtreeLedger,
	}, nil
}

type migratorRuntime struct {
	Snapshot         *util.PayloadSnapshot
	TransactionState state.NestedTransactionPreparer
	Interpreter      *interpreter.Interpreter
	Storage          *runtime.Storage
	Payloads         []*ledger.Payload
	Address          common.Address
	Accounts         *util.AccountsAtreeLedger
}

func (mr *migratorRuntime) GetReadOnlyStorage() *runtime.Storage {
	return runtime.NewStorage(util.NewPayloadsReadonlyLedger(mr.Snapshot), util.NopMemoryGauge{})
}
