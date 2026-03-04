package inspect

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution/computation"
	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/initialize"
	"github.com/onflow/flow-go/fvm/inspection"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/partial"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/model/verification/convert"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

func initStorages(
	lockManager lockctx.Manager,
	dataDir string,
	chunkDataPackDir string,
) (
	func() error,
	*store.All,
	storage.ChunkDataPacks,
	protocol.State,
	error,
) {
	db, err := common.InitStorage(dataDir)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not init storage database: %w", err)
	}

	storages := common.InitStorages(db)
	state, err := common.OpenProtocolState(lockManager, db, storages)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not open protocol state: %w", err)
	}

	// require the chunk data pack data must exist before returning the storage module
	chunkDataPackDB, err := storagepebble.ShouldOpenDefaultPebbleDB(
		log.Logger.With().Str("pebbledb", "cdp").Logger(), chunkDataPackDir)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not open chunk data pack DB: %w", err)
	}
	storedChunkDataPacks := store.NewStoredChunkDataPacks(metrics.NewNoopCollector(), pebbleimpl.ToDB(chunkDataPackDB), 1000)
	chunkDataPacks := store.NewChunkDataPacks(metrics.NewNoopCollector(),
		db, storedChunkDataPacks, storages.Collections, 1000)

	closer := func() error {
		var dbErr, chunkDataPackDBErr error

		if err := db.Close(); err != nil {
			dbErr = fmt.Errorf("failed to close protocol db: %w", err)
		}

		if err := chunkDataPackDB.Close(); err != nil {
			chunkDataPackDBErr = fmt.Errorf("failed to close chunk data pack db: %w", err)
		}
		return errors.Join(dbErr, chunkDataPackDBErr)
	}

	return closer, storages, chunkDataPacks, state, nil
}

// partialLedgerStorageSnapshot wraps a storage snapshot and tracks unknown register touches
type partialLedgerStorageSnapshot struct {
	snapshot snapshot.StorageSnapshot

	unknownRegTouch map[flow.RegisterID]struct{}
}

func (storage *partialLedgerStorageSnapshot) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	value, err := storage.snapshot.Get(id)
	if err != nil && errors.Is(err, ledger.ErrMissingKeys{}) {
		storage.unknownRegTouch[id] = struct{}{}
		return flow.RegisterValue{}, nil
	}

	return value, err
}

// chunkInspector handles the inspection of chunks by re-executing transactions
type chunkInspector struct {
	vm             fvm.VM
	vmCtx          fvm.Context
	systemChunkCtx fvm.Context
	logger         zerolog.Logger
	inspector      *inspection.TokenChanges
}

func newChunkInspector(
	logger zerolog.Logger,
	chainID flow.ChainID,
	headers storage.Headers,
	inspector *inspection.TokenChanges,
) *chunkInspector {
	vm := fvm.NewVirtualMachine()
	fvmOptions := initialize.InitFvmOptions(
		chainID,
		headers,
		false, // transaction fees not disabled
	)
	fvmOptions = append(
		[]fvm.Option{fvm.WithLogger(logger)},
		fvmOptions...,
	)

	fvmOptions = append(
		fvmOptions,
		computation.DefaultFVMOptions(
			chainID,
			false,
			false,
			false,
		)...,
	)
	vmCtx := fvm.NewContext(chainID.Chain(), fvmOptions...)

	return &chunkInspector{
		vm:             vm,
		vmCtx:          vmCtx,
		systemChunkCtx: vmCtx, // simplified for inspection
		logger:         logger,
		inspector:      inspector,
	}
}

// inspectChunk re-executes transactions in the chunk and runs the token inspector
func (ci *chunkInspector) inspectChunk(
	vc *verification.VerifiableChunkData,
) error {
	var transactions []*fvm.TransactionProcedure
	derivedBlockData := derived.NewEmptyDerivedBlockData(logical.Time(vc.TransactionOffset))

	ctx := fvm.NewContextFromParent(
		ci.vmCtx,
		fvm.WithBlockHeader(vc.Header),
		fvm.WithProtocolStateSnapshot(vc.Snapshot),
		fvm.WithDerivedBlockData(derivedBlockData),
	)

	if vc.IsSystemChunk {
		// For system chunks, create the system transaction
		txBody, err := blueprints.SystemChunkTransaction(ci.vmCtx.Chain)
		if err != nil {
			return fmt.Errorf("could not get system chunk transaction: %w", err)
		}
		transactions = []*fvm.TransactionProcedure{
			fvm.Transaction(txBody, vc.TransactionOffset),
		}
	} else {
		// For regular chunks, use the collection transactions
		transactions = make(
			[]*fvm.TransactionProcedure,
			0,
			len(vc.ChunkDataPack.Collection.Transactions))
		for i, txBody := range vc.ChunkDataPack.Collection.Transactions {
			tx := fvm.Transaction(txBody, vc.TransactionOffset+uint32(i))
			transactions = append(transactions, tx)
		}
	}

	// Construct partial trie from chunk data pack
	psmt, err := partial.NewLedger(
		vc.ChunkDataPack.Proof,
		ledger.State(vc.ChunkDataPack.StartState),
		partial.DefaultPathFinderVersion,
	)
	if err != nil {
		return fmt.Errorf("could not construct partial trie: %w", err)
	}

	// Create storage snapshot
	unknownRegTouch := make(map[flow.RegisterID]struct{})
	snapshotTree := snapshot.NewSnapshotTree(
		&partialLedgerStorageSnapshot{
			snapshot: executionState.NewLedgerStorageSnapshot(
				psmt,
				vc.ChunkDataPack.StartState),
			unknownRegTouch: unknownRegTouch,
		})

	// Execute each transaction and inspect
	for i, tx := range transactions {
		executionSnapshot, output, err := ci.vm.Run(
			ctx,
			tx,
			snapshotTree)
		if err != nil {
			ci.logger.Warn().
				Err(err).
				Int("tx_index", i).
				Hex("tx_id", tx.ID[:]).
				Msg("failed to execute transaction")
			continue
		}

		// Collect events for inspection
		events := make([]flow.Event, 0, len(output.Events)+len(output.ServiceEvents))
		events = append(events, output.Events...)
		events = append(events, output.ServiceEvents...)

		// Run the inspector
		result, err := ci.inspector.Inspect(ci.logger, snapshotTree, executionSnapshot, events)
		if err != nil {
			ci.logger.Warn().
				Err(err).
				Int("tx_index", i).
				Hex("tx_id", tx.ID[:]).
				Msg("failed to inspect transaction")
		} else {
			// Log the inspection result
			ci.logInspectionResult(tx.ID, i, result)
		}

		// Update snapshot tree for next transaction
		snapshotTree = snapshotTree.Append(executionSnapshot)
	}

	return nil
}

func (ci *chunkInspector) logInspectionResult(txID flow.Identifier, txIndex int, result inspection.Result) {
	if result == nil {
		ci.logger.Info().Msgf("no result from inspection, transaction did not trigger any token movements")
		return
	}

	lvl, evt := result.AsLogEvent()
	if evt == nil {
		ci.logger.Info().Msgf("transaction did not trigger any token movements")
		return
	}

	e := ci.logger.WithLevel(lvl).
		Hex("tx_id", txID[:]).
		Int("tx_index", txIndex)
	evt(e)
	e.Msg("Token inspection result")
}

// inspectChunkFromDataPack creates a VerifiableChunkData and inspects it
func inspectChunkFromDataPack(
	logger zerolog.Logger,
	chainID flow.ChainID,
	header *flow.Header,
	chunk *flow.Chunk,
	chunkDataPack *flow.ChunkDataPack,
	result *flow.ExecutionResult,
	protocolState protocol.State,
	headers storage.Headers,
	inspector *inspection.TokenChanges,
) error {
	// Get protocol snapshot at the block
	ps := protocolState.AtBlockID(header.ID())

	// Convert to verifiable chunk data
	vcd, err := convert.FromChunkDataPack(chunk, chunkDataPack, header, ps, result)
	if err != nil {
		return fmt.Errorf("could not convert chunk data pack: %w", err)
	}

	// Create chunk inspector and run inspection
	chunkInspector := newChunkInspector(logger, chainID, headers, inspector)
	return chunkInspector.inspectChunk(vcd)
}
