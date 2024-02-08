package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	pStorage "github.com/onflow/flow-go/storage/pebble"
)

// an increased limit for bootstrapping
const ledgerIntractionLimitNeededForBootstrapping = 1_000_000_000

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
	chain flow.Chain,
	opts ...fvm.BootstrapProcedureOption,
) (flow.StateCommitment, error) {
	startCommit := flow.StateCommitment(ledger.InitialState())
	storageSnapshot := state.NewLedgerStorageSnapshot(
		ledger,
		startCommit)

	vm := fvm.NewVirtualMachine()

	ctx := fvm.NewContext(
		fvm.WithLogger(b.logger),
		fvm.WithMaxStateInteractionSize(ledgerIntractionLimitNeededForBootstrapping),
		fvm.WithChain(chain),
		fvm.WithEVMEnabled(true),
	)

	bootstrap := fvm.Bootstrap(
		servicePublicKey,
		opts...,
	)

	executionSnapshot, _, err := vm.Run(ctx, bootstrap, storageSnapshot)
	if err != nil {
		return flow.DummyStateCommitment, err
	}

	newStateCommitment, _, _, err := state.CommitDelta(
		ledger,
		executionSnapshot,
		storehouse.NewExecutingBlockSnapshot(storageSnapshot, startCommit),
	)
	if err != nil {
		return flow.DummyStateCommitment, err
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
		return flow.DummyStateCommitment, false, nil
	}

	if err != nil {
		return flow.DummyStateCommitment, false, err
	}

	return commit, true, nil
}

func (b *Bootstrapper) BootstrapExecutionDatabase(
	db *badger.DB,
	rootSeal *flow.Seal,
) error {

	commit := rootSeal.FinalState
	err := operation.RetryOnConflict(db.Update, func(txn *badger.Txn) error {

		err := operation.InsertExecutedBlock(rootSeal.BlockID)(txn)
		if err != nil {
			return fmt.Errorf("could not index initial genesis execution block: %w", err)
		}

		err = operation.SkipDuplicates(operation.IndexExecutionResult(rootSeal.BlockID, rootSeal.ResultID))(txn)
		if err != nil {
			return fmt.Errorf("could not index result for root result: %w", err)
		}

		err = operation.IndexStateCommitment(flow.ZeroID, commit)(txn)
		if err != nil {
			return fmt.Errorf("could not index void state commitment: %w", err)
		}

		err = operation.IndexStateCommitment(rootSeal.BlockID, commit)(txn)
		if err != nil {
			return fmt.Errorf("could not index genesis state commitment: %w", err)
		}

		snapshots := make([]*snapshot.ExecutionSnapshot, 0)
		err = operation.InsertExecutionStateInteractions(rootSeal.BlockID, snapshots)(txn)
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

func ImportRegistersFromCheckpoint(logger zerolog.Logger, checkpointFile string, checkpointHeight uint64, checkpointRootHash ledger.RootHash, pdb *pebble.DB, workerCount int) error {
	logger.Info().Msgf("importing registers from checkpoint file %s at height %d with root hash: %v", checkpointFile, checkpointHeight, checkpointRootHash)

	bootstrap, err := pStorage.NewRegisterBootstrap(pdb, checkpointFile, checkpointHeight, checkpointRootHash, logger)
	if err != nil {
		return fmt.Errorf("could not create registers bootstrapper: %w", err)
	}

	// TODO: find a way to hook a context up to this to allow a graceful shutdown
	err = bootstrap.IndexCheckpointFile(context.Background(), workerCount)
	if err != nil {
		return fmt.Errorf("could not load checkpoint file: %w", err)
	}

	logger.Info().Msgf("finish importing registers from checkpoint file %s at height %d", checkpointFile, checkpointHeight)
	return nil
}
