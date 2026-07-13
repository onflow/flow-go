package inspect

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
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
		_ = db.Close()
		return nil, nil, nil, nil, fmt.Errorf("could not open protocol state: %w", err)
	}

	// require the chunk data pack data must exist before returning the storage module
	chunkDataPackDB, err := storagepebble.ShouldOpenDefaultPebbleDB(
		log.Logger.With().Str("pebbledb", "cdp").Logger(), chunkDataPackDir)
	if err != nil {
		_ = db.Close()
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
	callbackCtx    fvm.Context
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
		systemChunkCtx: computer.SystemChunkContext(vmCtx, metrics.NewNoopCollector()),
		callbackCtx:    computer.ScheduledTransactionContext(vmCtx, metrics.NewNoopCollector()),
		logger:         logger,
		inspector:      inspector,
	}
}

// inspectChunk re-executes transactions in the chunk and runs the token inspector.
// For system chunks, this mirrors the verification node's behavior by using the proper
// system chunk FVM context and handling scheduled callback transactions.
func (ci *chunkInspector) inspectChunk(
	vc *verification.VerifiableChunkData,
) error {
	var transactions []*fvm.TransactionProcedure
	derivedBlockData := derived.NewEmptyDerivedBlockData(logical.Time(vc.TransactionOffset))

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

	// Set up the appropriate FVM context and transactions based on chunk type
	var ctx fvm.Context
	var callbackCtx fvm.Context
	var processAlreadyExecuted bool

	if vc.IsSystemChunk {
		ctx = fvm.NewContextFromParent(
			ci.systemChunkCtx,
			fvm.WithBlockHeader(vc.Header),
			fvm.WithProtocolStateSnapshot(vc.Snapshot),
			fvm.WithDerivedBlockData(derivedBlockData),
		)
		callbackCtx = fvm.NewContextFromParent(
			ci.callbackCtx,
			fvm.WithBlockHeader(vc.Header),
			fvm.WithProtocolStateSnapshot(vc.Snapshot),
			fvm.WithDerivedBlockData(derivedBlockData),
		)

		transactions, processAlreadyExecuted, err = ci.createSystemChunkTransactions(
			callbackCtx, &snapshotTree, vc.TransactionOffset)
		if err != nil {
			return fmt.Errorf("could not create system chunk transactions: %w", err)
		}
	} else {
		ctx = fvm.NewContextFromParent(
			ci.vmCtx,
			fvm.WithBlockHeader(vc.Header),
			fvm.WithProtocolStateSnapshot(vc.Snapshot),
			fvm.WithDerivedBlockData(derivedBlockData),
		)

		transactions = make(
			[]*fvm.TransactionProcedure,
			0,
			len(vc.ChunkDataPack.Collection.Transactions))
		for i, txBody := range vc.ChunkDataPack.Collection.Transactions {
			tx := fvm.Transaction(txBody, vc.TransactionOffset+uint32(i))
			transactions = append(transactions, tx)
		}
	}

	// Execute each transaction and inspect.
	// For system chunks with scheduled transactions enabled, the process callback
	// transaction (index 0) was already executed by createSystemChunkTransactions,
	// so we start from index 1.
	txStartIndex := 0
	if processAlreadyExecuted {
		txStartIndex = 1
	}

	for i := txStartIndex; i < len(transactions); i++ {
		tx := transactions[i]
		txCtx := ctx

		// For system chunks with scheduled transactions, use callbackCtx for all
		// transactions except the last (the system transaction itself).
		if vc.IsSystemChunk && ci.vmCtx.ScheduledTransactionsEnabled && i < len(transactions)-1 {
			txCtx = callbackCtx
		}

		ci.logger.Info().
			Int("tx_index", i).
			Hex("tx_id", tx.ID[:]).
			Msg("executing transaction")

		executionSnapshot, output, err := ci.vm.Run(
			txCtx,
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
			ci.logInspectionResult(tx.ID, i, result)
		}

		// Update snapshot tree for next transaction
		snapshotTree = snapshotTree.Append(executionSnapshot)
	}

	return nil
}

// createSystemChunkTransactions builds the transaction list for a system chunk,
// mirroring the verification node's ChunkVerifier.createSystemChunk logic.
// If scheduled transactions are disabled, returns only the system transaction.
// If enabled, executes the process callback transaction, generates execute callback
// transactions from its events, and appends the system transaction last.
// Returns the transaction list and whether the process callback was already executed
// (requiring the caller to skip it in the execution loop).
func (ci *chunkInspector) createSystemChunkTransactions(
	callbackCtx fvm.Context,
	snapshotTree *snapshot.SnapshotTree,
	transactionOffset uint32,
) ([]*fvm.TransactionProcedure, bool, error) {
	txIndex := transactionOffset

	// If scheduled transactions are disabled, only the system transaction is in the chunk
	if !ci.vmCtx.ScheduledTransactionsEnabled {
		txBody, err := blueprints.SystemChunkTransaction(ci.vmCtx.Chain)
		if err != nil {
			return nil, false, fmt.Errorf("could not get system chunk transaction: %w", err)
		}
		return []*fvm.TransactionProcedure{
			fvm.Transaction(txBody, txIndex),
		}, false, nil
	}

	// Execute process callback transaction to discover scheduled transactions
	processBody, err := blueprints.ProcessCallbacksTransaction(ci.vmCtx.Chain)
	if err != nil {
		return nil, false, fmt.Errorf("could not get process callback transaction: %w", err)
	}
	processTx := fvm.Transaction(processBody, txIndex)

	executionSnapshot, processOutput, err := ci.vm.Run(callbackCtx, processTx, *snapshotTree)
	if err != nil {
		return nil, false, fmt.Errorf("failed to execute process callback transaction: %w", err)
	}

	// Generate callback execution transactions from the events
	callbackTxs, err := blueprints.ExecuteCallbacksTransactions(ci.vmCtx.Chain, processOutput.Events)
	if err != nil {
		return nil, false, fmt.Errorf("failed to generate callback execution transactions: %w", err)
	}

	// Build the final transaction list: [processCallback, ...callbackExecutions, systemTx]
	transactions := make([]*fvm.TransactionProcedure, 0, len(callbackTxs)+2)
	transactions = append(transactions, processTx)

	for _, c := range callbackTxs {
		txIndex++
		transactions = append(transactions, fvm.Transaction(c, txIndex))
	}

	systemTx, err := blueprints.SystemChunkTransaction(ci.vmCtx.Chain)
	if err != nil {
		return nil, false, fmt.Errorf("could not get system chunk transaction: %w", err)
	}
	txIndex++
	transactions = append(transactions, fvm.Transaction(systemTx, txIndex))

	// Update snapshot tree with the process callback execution
	*snapshotTree = snapshotTree.Append(executionSnapshot)

	return transactions, true, nil
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
