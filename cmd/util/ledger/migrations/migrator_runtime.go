package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type migrationTransactionPreparer struct {
	state.NestedTransactionPreparer
	derived.DerivedTransactionPreparer
}

var _ storage.TransactionPreparer = migrationTransactionPreparer{}

// migratorRuntime is a runtime that can be used to run a migration on a single account
func NewMigratorRuntime(
	address common.Address,
	payloads []*ledger.Payload,
	config util.RuntimeInterfaceConfig,
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
	runtimeStorage := runtime.NewStorage(accountsAtreeLedger, util.NopMemoryGauge{})

	derivedChainData, err := derived.NewDerivedChainData(derived.DefaultDerivedDataCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create derived chain data: %w", err)
	}

	// The current block ID does not matter here, it is only for keeping a cross-block cache, which is not needed here.
	derivedTransactionData := derivedChainData.
		NewDerivedBlockDataForScript(flow.Identifier{}).
		NewSnapshotReadDerivedTransactionData()

	ri := &util.MigrationRuntimeInterface{
		Accounts: accounts,
		Programs: environment.NewPrograms(
			tracing.NewTracerSpan(),
			util.NopMeter{},
			environment.NoopMetricsReporter{},
			migrationTransactionPreparer{
				NestedTransactionPreparer:  transactionState,
				DerivedTransactionPreparer: derivedTransactionData,
			},
			accounts,
		),
		RuntimeInterfaceConfig: config,
		ProgramErrors:          map[common.Location]error{},
	}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{
		// Attachments are enabled everywhere except for Mainnet
		AttachmentsEnabled: true,
	})

	env.Configure(
		ri,
		runtime.NewCodesAndPrograms(),
		runtimeStorage,
		nil,
	)

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		env.InterpreterConfig,
	)
	if err != nil {
		return nil, err
	}

	return &migratorRuntime{
		Address:                 address,
		Payloads:                payloads,
		Snapshot:                snapshot,
		TransactionState:        transactionState,
		Interpreter:             inter,
		Storage:                 runtimeStorage,
		AccountsLedger:          accountsAtreeLedger,
		Accounts:                accounts,
		ContractAdditionHandler: env,
		ContractNamesProvider:   env,
	}, nil
}

type migratorRuntime struct {
	Snapshot                util.MigrationStorageSnapshot
	TransactionState        state.NestedTransactionPreparer
	Interpreter             *interpreter.Interpreter
	Storage                 *runtime.Storage
	Payloads                []*ledger.Payload
	Address                 common.Address
	AccountsLedger          *util.AccountsAtreeLedger
	Accounts                environment.Accounts
	ContractAdditionHandler stdlib.AccountContractAdditionHandler
	ContractNamesProvider   stdlib.AccountContractNamesProvider
}

var _ stdlib.AccountContractNamesProvider = &migratorRuntime{}

func (mr *migratorRuntime) GetReadOnlyStorage() *runtime.Storage {
	return runtime.NewStorage(util.NewPayloadsReadonlyLedger(mr.Snapshot), util.NopMemoryGauge{})
}

func (mr *migratorRuntime) GetAccountContractNames(address common.Address) ([]string, error) {
	return mr.Accounts.GetContractNames(flow.Address(address))
}
