package common

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"

	ledger "github.com/onflow/flow-go/ledger/complete"
	protocolbadger "github.com/onflow/flow-go/state/protocol/badger"
	storagebadger "github.com/onflow/flow-go/storage/badger"
)

// InitProtocolState ...
func InitProtocolState(db *badger.DB, storages *storage.All) (protocol.State, error) {
	metrics := &metrics.NoopCollector{}
	tracer := trace.NewNoopTracer()
	distributor := events.NewDistributor()

	protocolState, err := protocolbadger.NewState(
		metrics,
		tracer,
		db,
		storages.Headers,
		storages.Seals,
		storages.Index,
		storages.Payloads,
		storages.Blocks,
		storages.Setups,
		storages.EpochCommits,
		storages.Statuses,
		distributor,
	)

	if err != nil {
		return nil, fmt.Errorf("could not init protocol state: %w", err)
	}

	return protocolState, nil
}

// InitForest ...
func InitForest(executionDir string) (*mtrie.Forest, error) {
	w, err := wal.NewWAL(
		nil,
		nil,
		executionDir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return nil, err
	}

	forest, err := mtrie.NewForest(pathfinder.PathByteSize, executionDir, complete.DefaultCacheSize, metrics.NewNoopCollector(), nil)
	if err != nil {
		return nil, err
	}

	err = w.ReplayOnForest(forest)
	if err != nil {
		return nil, err
	}

	return forest, nil
}

// InitStates ...
func InitStates(db *badger.DB, executionStateDir string) (protocol.State, state.ExecutionState, error) {
	metrics := &metrics.NoopCollector{}
	tracer := trace.NewNoopTracer()

	storage := InitStorages(db)

	protocolState, err := InitProtocolState(db, storage)
	if err != nil {
		return nil, nil, fmt.Errorf("could not init protocol state: %w", err)
	}

	results := storagebadger.NewExecutionResults(db)
	receipts := storagebadger.NewExecutionReceipts(db, results)
	chunkDataPacks := storagebadger.NewChunkDataPacks(db)
	stateCommitments := storagebadger.NewCommits(metrics, db)
	transactions := storagebadger.NewTransactions(metrics, db)
	collections := storagebadger.NewCollections(db, transactions)

	ledgerStorage, err := ledger.NewLedger(executionStateDir, 100, metrics, zerolog.Nop(), nil, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("could not init ledger: %w", err)
	}

	executionState := state.NewExecutionState(
		ledgerStorage,
		stateCommitments,
		storage.Blocks,
		collections,
		chunkDataPacks,
		results,
		receipts,
		db,
		tracer,
	)

	return protocolState, executionState, nil
}
