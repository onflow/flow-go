package run

import (
	"fmt"
	"math"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

const bootstrapCheckpointFile = "bootstrap-checkpoint"

func GenerateExecutionState(
	dbDir string,
	accountKey flow.AccountPublicKey,
	chain flow.Chain,
	bootstrapOptions ...fvm.BootstrapProcedureOption,
) (flow.StateCommitment, error) {
	const (
		capacity           = 100
		checkpointDistance = math.MaxInt // A large number to prevent last creation.
		checkpointsToKeep  = 1
	)

	metricsCollector := &metrics.NoopCollector{}

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dbDir, capacity, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return flow.DummyStateCommitment, err
	}

	ledgerStorage, err := ledger.NewLedger(diskWal, capacity, metricsCollector, zerolog.Nop(), ledger.DefaultPathFinderVersion)
	if err != nil {
		return flow.DummyStateCommitment, err
	}

	compactor, err := complete.NewCompactor(ledgerStorage, diskWal, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
	if err != nil {
		return flow.DummyStateCommitment, err
	}
	<-compactor.Ready()

	defer func() {
		<-ledgerStorage.Done()
		<-compactor.Done()
	}()

	stateCommitment, err := bootstrap.
		NewBootstrapper(zerolog.Nop()).
		BootstrapLedger(
			ledgerStorage,
			accountKey,
			chain,
			bootstrapOptions...,
		)
	if err != nil {
		return flow.DummyStateCommitment, err
	}

	checkpointer, err := ledgerStorage.Checkpointer()
	if err != nil {
		return flow.DummyStateCommitment, err
	}

	root := 0
	err = checkpointer.Checkpoint(root)
	if err != nil {
		return flow.DummyStateCommitment, err
	}

	trie, err := checkpointer.LoadCheckpoint(root)
	if err != nil {
		return flow.DummyStateCommitment, err
	}

	err = wal.StoreCheckpointV6(trie, dbDir, bootstrapCheckpointFile, zerolog.Nop(), 1)
	if err != nil {
		return flow.DummyStateCommitment, fmt.Errorf("failed to store bootstrap checkpoint: %w", err)
	}

	return stateCommitment, nil
}
