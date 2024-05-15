package migrations

import (
	"fmt"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	evmStdlib "github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
)

type BasicMigrationRuntime struct {
	TransactionState state.NestedTransactionPreparer
	Storage          *runtime.Storage
	AccountsLedger   *util.AccountsAtreeLedger
	Accounts         environment.Accounts
}

type InterpreterMigrationRuntime struct {
	*BasicMigrationRuntime
	Interpreter             *interpreter.Interpreter
	ContractAdditionHandler stdlib.AccountContractAdditionHandler
	ContractNamesProvider   stdlib.AccountContractNamesProvider
}

// InterpreterMigrationRuntimeConfig is used to configure the InterpreterMigrationRuntime.
// The code, contract names, and program loading functions can be nil,
// in which case program loading will be configured to use the default behavior,
// loading contracts from the given payloads.
// The listener function is optional and can be used to listen for program loading events.
type InterpreterMigrationRuntimeConfig struct {
	GetCode                  util.GetContractCodeFunc
	GetContractNames         util.GetContractNamesFunc
	GetOrLoadProgram         util.GetOrLoadProgramFunc
	GetOrLoadProgramListener util.GerOrLoadProgramListenerFunc
}

func (c InterpreterMigrationRuntimeConfig) NewRuntimeInterface(
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
		)
		if err != nil {
			return nil, err
		}
	}

	return util.NewMigrationRuntimeInterface(
		getCodeFunc,
		getContractNames,
		getOrLoadProgram,
		c.GetOrLoadProgramListener,
	), nil
}

// NewBasicMigrationRuntime returns a basic runtime for migrations.
func NewBasicMigrationRuntime(regs registers.Registers) *BasicMigrationRuntime {
	transactionState := state.NewTransactionStateFromExecutionState(
		state.NewExecutionStateWithSpockStateHasher(
			registers.StorageSnapshot{
				Registers: regs,
			},
			state.DefaultParameters(),
			func() hash.Hasher {
				return newDummyHasher(32)
			},
		),
	)
	accounts := environment.NewAccounts(transactionState)

	accountsAtreeLedger := util.NewAccountsAtreeLedger(accounts)
	runtimeStorage := runtime.NewStorage(accountsAtreeLedger, nil)

	return &BasicMigrationRuntime{
		TransactionState: transactionState,
		Storage:          runtimeStorage,
		AccountsLedger:   accountsAtreeLedger,
		Accounts:         accounts,
	}
}

// NewInterpreterMigrationRuntime returns a runtime for migrations that need an interpreter.
func NewInterpreterMigrationRuntime(
	regs registers.Registers,
	chainID flow.ChainID,
	config InterpreterMigrationRuntimeConfig,
) (
	*InterpreterMigrationRuntime,
	error,
) {
	basicMigrationRuntime := NewBasicMigrationRuntime(regs)

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{
		AttachmentsEnabled: true,
	})

	runtimeInterface, err := config.NewRuntimeInterface(
		basicMigrationRuntime.TransactionState,
		basicMigrationRuntime.Accounts,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime interface: %w", err)
	}

	evmContractAccountAddress, err := evm.ContractAccountAddress(chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get EVM contract account address for chain %s: %w", chainID, err)
	}

	evmStdlib.SetupEnvironment(env, nil, evmContractAccountAddress)

	env.Configure(
		runtimeInterface,
		runtime.NewCodesAndPrograms(),
		basicMigrationRuntime.Storage,
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

	return &InterpreterMigrationRuntime{
		BasicMigrationRuntime:   basicMigrationRuntime,
		Interpreter:             inter,
		ContractAdditionHandler: env,
		ContractNamesProvider:   env,
	}, nil
}

type dummyHasher struct{ size int }

func newDummyHasher(size int) hash.Hasher               { return &dummyHasher{size} }
func (d *dummyHasher) Algorithm() hash.HashingAlgorithm { return hash.UnknownHashingAlgorithm }
func (d *dummyHasher) Size() int                        { return d.size }
func (d *dummyHasher) ComputeHash([]byte) hash.Hash     { return make([]byte, d.size) }
func (d *dummyHasher) Write([]byte) (int, error)        { return 0, nil }
func (d *dummyHasher) SumHash() hash.Hash               { return make([]byte, d.size) }
func (d *dummyHasher) Reset()                           {}
