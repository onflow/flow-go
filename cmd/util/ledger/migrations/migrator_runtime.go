package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type MigratorRuntime struct {
	Snapshot                *util.PayloadSnapshot
	TransactionState        state.NestedTransactionPreparer
	Interpreter             *interpreter.Interpreter
	Storage                 *runtime.Storage
	Payloads                []*ledger.Payload
	AccountsLedger          *util.AccountsAtreeLedger
	Accounts                environment.Accounts
	ContractAdditionHandler stdlib.AccountContractAdditionHandler
	ContractNamesProvider   stdlib.AccountContractNamesProvider
}

// MigratorRuntimeConfig is used to configure the MigratorRuntime.
// The code, contract names, and program loading functions can be nil,
// in which case program loading will be configured to use the default behavior,
// loading contracts from the given payloads.
// The listener function is optional and can be used to listen for program loading events.
type MigratorRuntimeConfig struct {
	GetCode                  util.GetContractCodeFunc
	GetContractNames         util.GetContractNamesFunc
	GetOrLoadProgram         util.GetOrLoadProgramFunc
	GetOrLoadProgramListener util.GerOrLoadProgramListenerFunc
}

func (c MigratorRuntimeConfig) NewRuntimeInterface(
	transactionState state.NestedTransactionPreparer,
	accounts environment.Accounts,
) (
	runtime.Interface,
	error,
) {

	getCodeFunc := func(location common.AddressLocation) ([]byte, error) {
		// First, try to get the code from the provided function.
		// If it is not provided, fall back to the default behavior,
		// getting the code from the accounts.

		getCodeFunc := c.GetCode
		if getCodeFunc != nil {
			code, err := getCodeFunc(location)
			if err != nil || code != nil {
				// If the code was found, or if an error occurred, then return.
				return code, err
			}
		}

		return accounts.GetContract(
			location.Name,
			flow.Address(location.Address),
		)
	}

	getContractNames := c.GetContractNames
	if getContractNames == nil {
		getContractNames = accounts.GetContractNames
	}

	getOrLoadProgram := c.GetOrLoadProgram
	if getOrLoadProgram == nil {
		var err error
		getOrLoadProgram, err = util.NewProgramsGetOrLoadProgramFunc(
			transactionState,
			accounts,
			c.GetOrLoadProgramListener,
		)
		if err != nil {
			return nil, err
		}
	}

	return util.NewMigrationRuntimeInterface(
		getCodeFunc,
		getContractNames,
		getOrLoadProgram,
	), nil
}

// NewMigratorRuntime returns a runtime that can be used in migrations.
func NewMigratorRuntime(
	payloads []*ledger.Payload,
	config MigratorRuntimeConfig,
) (
	*MigratorRuntime,
	error,
) {
	snapshot, err := util.NewPayloadSnapshot(payloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload snapshot: %w", err)
	}

	transactionState := state.NewTransactionState(snapshot, state.DefaultParameters())
	accounts := environment.NewAccounts(transactionState)

	accountsAtreeLedger := util.NewAccountsAtreeLedger(accounts)
	runtimeStorage := runtime.NewStorage(accountsAtreeLedger, nil)

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{
		AttachmentsEnabled: true,
	})

	runtimeInterface, err := config.NewRuntimeInterface(transactionState, accounts)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime interface: %w", err)
	}

	env.Configure(
		runtimeInterface,
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

	return &MigratorRuntime{
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
