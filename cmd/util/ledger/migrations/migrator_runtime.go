package migrations

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	util2 "github.com/onflow/flow-go/module/util"
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

	ri.GetOrLoadProgramFunc = func(
		location runtime.Location,
		load func() (*interpreter.Program, error),
	) (*interpreter.Program, error) {
		return load()
	}

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
		AccountsLedger:   accountsAtreeLedger,
		Accounts:         accounts,
	}, nil
}

type migratorRuntime struct {
	Snapshot         *util.PayloadSnapshot
	TransactionState state.NestedTransactionPreparer
	Interpreter      *interpreter.Interpreter
	Storage          *runtime.Storage
	Payloads         []*ledger.Payload
	Address          common.Address
	AccountsLedger   *util.AccountsAtreeLedger
	Accounts         environment.Accounts
}

func (mr *migratorRuntime) GetReadOnlyStorage() *runtime.Storage {
	return runtime.NewStorage(util.NewPayloadsReadonlyLedger(mr.Snapshot), util.NopMemoryGauge{})
}

func (mr *migratorRuntime) ChildInterpreters(log zerolog.Logger, n int, address flow.Address) ([]*interpreter.Interpreter, func() error, error) {

	interpreters := make([]*interpreter.Interpreter, n)
	storages := make([]*runtime.Storage, n)

	//id := flow.AccountStatusRegisterID(address)
	//statusBytes, err := mr.Accounts.GetValue([]byte(id.Owner), []byte(id.Key))
	//if err != nil {
	//	return nil, nil, fmt.Errorf(
	//		"failed to load account status for the account (%s): %w",
	//		address.String(),
	//		err)
	//}
	//accountStatus, err := environment.AccountStatusFromBytes(statusBytes)
	//if err != nil {
	//	return nil, nil, fmt.Errorf(
	//		"failed to parse account status for the account (%s): %w",
	//		address.String(),
	//		err)
	//}
	//
	//index := accountStatus.StorageIndex()
	//index = index.Next()

	//// create a channel that will generate storage indexes on demand
	//storageIndexChan := make(chan atree.StorageIndex)
	//stopStorageIndexChan := make(chan struct{})
	//go func() {
	//	for {
	//		select {
	//		case <-stopStorageIndexChan:
	//			return
	//		case storageIndexChan <- index:
	//			index = index.Next()
	//		}
	//	}
	//}()

	mu := sync.Mutex{}

	for i := 0; i < n; i++ {
		ledger := util.NewPayloadsReadonlyLedger(mr.Snapshot)
		ledger.AllocateStorageIndexFunc = func(owner []byte) (atree.StorageIndex, error) {
			mu.Lock()
			defer mu.Unlock()

			return mr.AccountsLedger.AllocateStorageIndex(owner)

		}
		ledger.SetValueFunc = func(owner, key, value []byte) (err error) {
			return mr.AccountsLedger.SetValue(owner, key, value)
		}

		storage := runtime.NewStorage(ledger, util.NopMemoryGauge{})

		ri := &util.MigrationRuntimeInterface{}

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
			return nil, nil, err
		}

		interpreters[i] = inter
		storages[i] = storage
	}

	closeInterpreters := func() error {
		progressLog := util2.LogProgress(
			log,
			util2.DefaultLogProgressConfig(
				"closing child interpreters",
				len(interpreters),
			))
		//index := <-storageIndexChan
		//close(stopStorageIndexChan)
		//accountStatus.SetStorageIndex(index)
		//err := mr.Accounts.SetValue([]byte(id.Owner), []byte(id.Key), accountStatus.ToBytes())
		//if err != nil {
		//	return fmt.Errorf(
		//		"failed to save account status for the account (%s): %w",
		//		address.String(),
		//		err)
		//}

		for i, storage := range storages {
			err := storage.Commit(interpreters[i], false)
			if err != nil {
				return fmt.Errorf("failed to commit storage: %w", err)
			}
			progressLog(1)
		}

		return nil
	}

	return interpreters, closeInterpreters, nil
}
