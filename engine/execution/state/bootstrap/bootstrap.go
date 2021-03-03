package bootstrap

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type Bootstrapper struct {
	logger zerolog.Logger
}

func NewBootstrapper(logger zerolog.Logger) *Bootstrapper {
	return &Bootstrapper{
		logger: logger,
	}
}

// BootstrapLedger adds the above root account to the ledger and initializes execution node-only data
func (b *Bootstrapper) BootstrapLedger(
	ledger ledger.Ledger,
	servicePublicKey flow.AccountPublicKey,
	initialTokenSupply cadence.UFix64,
	chain flow.Chain,
) (flow.StateCommitment, error) {
	view := delta.NewView(state.LedgerGetRegister(ledger, ledger.InitialState()))
	programs := fvm.NewEmptyPrograms()

	vm := fvm.New(runtime.NewInterpreterRuntime())

	ctx := fvm.NewContext(b.logger, fvm.WithChain(chain))

	bootstrap := fvm.Bootstrap(
		servicePublicKey,
		fvm.WithInitialTokenSupply(initialTokenSupply),
	)

	err := vm.Run(ctx, bootstrap, view, programs)
	if err != nil {
		return nil, err
	}

	newStateCommitment, err := state.CommitDelta(ledger, view.Delta(), ledger.InitialState())
	if err != nil {
		return nil, err
	}

	return newStateCommitment, nil
}

// IsBootstrapped returns whether the execution database has been bootstrapped, if yes, returns the
// root statecommitment
func (b *Bootstrapper) IsBootstrapped(db *badger.DB) (flow.StateCommitment, bool, error) {
	var commit flow.StateCommitment

	err := db.View(func(txn *badger.Txn) error {
		err := operation.LookupStateCommitment(flow.ZeroID, &commit)(txn)
		if err != nil {
			return fmt.Errorf("could not lookup state commitment: %w", err)
		}

		return nil
	})

	if errors.Is(err, storage.ErrNotFound) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	return commit, true, nil
}

func (b *Bootstrapper) BootstrapExecutionDatabase(db *badger.DB, commit flow.StateCommitment, genesis *flow.Header) error {

	err := operation.RetryOnConflict(db.Update, func(txn *badger.Txn) error {

		err := operation.InsertExecutedBlock(genesis.ID())(txn)
		if err != nil {
			return fmt.Errorf("could not index initial genesis execution block: %w", err)
		}

		err = operation.IndexStateCommitment(flow.ZeroID, commit)(txn)
		if err != nil {
			return fmt.Errorf("could not index void state commitment: %w", err)
		}

		err = operation.IndexStateCommitment(genesis.ID(), commit)(txn)
		if err != nil {
			return fmt.Errorf("could not index genesis state commitment: %w", err)
		}

		views := make([]*delta.Snapshot, 0)
		err = operation.InsertExecutionStateInteractions(genesis.ID(), views)(txn)
		if err != nil {
			return fmt.Errorf("could not bootstrap execution state interactions: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
