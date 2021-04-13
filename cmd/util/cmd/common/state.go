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
	"github.com/onflow/flow-go/storage"

	protocolbadger "github.com/onflow/flow-go/state/protocol/badger"
	storagebadger "github.com/onflow/flow-go/storage/badger"
)

func InitProtocolState(db *badger.DB, storages *storage.All) (protocol.State, error) {
	metrics := &metrics.NoopCollector{}

	protocolState, err := protocolbadger.OpenState(
		metrics,
		db,
		storages.Headers,
		storages.Seals,
		storages.Results,
		storages.Blocks,
		storages.Setups,
		storages.EpochCommits,
		storages.Statuses,
	)

	if err != nil {
		return nil, fmt.Errorf("could not init protocol state: %w", err)
	}

	return protocolState, nil
}

// InitForest ...
func InitForest(executionDir string) (*mtrie.Forest, error) {
	metrics := &metrics.NoopCollector{}

	w, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		metrics,
		executionDir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return nil, err
	}

	forest, err := mtrie.NewForest(pathfinder.PathByteSize, complete.DefaultCacheSize, metrics, nil)
	if err != nil {
		return nil, err
	}

	err = w.ReplayOnForest(forest)
	if err != nil {
		return nil, err
	}

	return forest, nil
}

// InitStates initializes the protocol and execution states
func InitStates(db *badger.DB, executionStateDir string) (protocol.State, state.ExecutionState, error) {
	metrics := &metrics.NoopCollector{}
	tracer := trace.NewNoopTracer()

	storage := InitStorages(db)

	protocolState, err := InitProtocolState(db, storage)
	if err != nil {
		return nil, nil, fmt.Errorf("could not init protocol state: %w", err)
	}

	results := storagebadger.NewExecutionResults(metrics, db)
	receipts := storagebadger.NewExecutionReceipts(metrics, db, results)
	chunkDataPacks := storagebadger.NewChunkDataPacks(db)
	stateCommitments := storagebadger.NewCommits(metrics, db)
	transactions := storagebadger.NewTransactions(metrics, db)
	collections := storagebadger.NewCollections(db, transactions)
	myReceipts := storagebadger.NewMyExecutionReceipts(metrics, db, receipts)
	events := storagebadger.NewEvents(metrics, db)
	serviceEvents := storagebadger.NewServiceEvents(metrics, db)
	transactionResults := storagebadger.NewTransactionResults(metrics, db, 10000)

	wal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics, executionStateDir, complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return nil, nil, fmt.Errorf("could not init disk WAL: %w", err)
	}

	ledgerStorage, err := complete.NewLedger(wal, 100, metrics, zerolog.Nop(), 0)
	if err != nil {
		return nil, nil, fmt.Errorf("could not init ledger: %w", err)
	}

	executionState := state.NewExecutionState(
		ledgerStorage,
		stateCommitments,
		storage.Blocks,
		storage.Headers,
		collections,
		chunkDataPacks,
		results,
		receipts,
		myReceipts,
		events,
		serviceEvents,
		transactionResults,
		db,
		tracer,
	)

	return protocolState, executionState, nil
}
