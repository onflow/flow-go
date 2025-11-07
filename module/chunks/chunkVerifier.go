package chunks

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/computation/computer"
	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	fvmState "github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/partial"
	chmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/metrics"
)

// ChunkVerifier is a verifier based on the current definitions of the flow network
type ChunkVerifier struct {
	vm             fvm.VM
	vmCtx          fvm.Context
	systemChunkCtx fvm.Context
	callbackCtx    fvm.Context
	logger         zerolog.Logger
}

// NewChunkVerifier creates a chunk verifier containing a flow virtual machine
func NewChunkVerifier(vm fvm.VM, vmCtx fvm.Context, logger zerolog.Logger) *ChunkVerifier {
	return &ChunkVerifier{
		vm:             vm,
		vmCtx:          vmCtx,
		systemChunkCtx: computer.SystemChunkContext(vmCtx, metrics.NewNoopCollector()),
		callbackCtx:    computer.CallbackContext(vmCtx, metrics.NewNoopCollector()),
		logger:         logger.With().Str("component", "chunk_verifier").Logger(),
	}
}

// Verify verifies a given VerifiableChunk by executing it and checking the
// final state commitment.
// It returns a Spock Secret as a byte array, verification fault of the chunk,
// and an error.
func (fcv *ChunkVerifier) Verify(
	vc *verification.VerifiableChunkData,
) (
	[]byte,
	error,
) {
	fcv.logger.Info().
		Uint64("chunk_index", vc.Chunk.Index).
		Uint32("transaction_offset", vc.TransactionOffset).
		Bool("is_system_chunk", vc.IsSystemChunk).
		Str("chunk_id", vc.Chunk.ID().String()).
		Str("block_id", vc.Chunk.BlockID.String()).
		Msg("Verify: starting chunk verification")

	var ctx fvm.Context
	var callbackCtx fvm.Context
	var transactions []*fvm.TransactionProcedure

	fcv.logger.Debug().
		Uint64("chunk_index", vc.Chunk.Index).
		Msg("Verify: creating derivedBlockData")
	derivedBlockData := derived.NewEmptyDerivedBlockData(logical.Time(vc.TransactionOffset))
	fcv.logger.Debug().
		Uint64("chunk_index", vc.Chunk.Index).
		Msg("Verify: derivedBlockData created")

	if vc.IsSystemChunk {
		fcv.logger.Debug().
			Uint64("chunk_index", vc.Chunk.Index).
			Msg("Verify: creating contexts for system chunk")
		ctx = contextFromVerifiableChunk(fcv.systemChunkCtx, vc, derivedBlockData)
		callbackCtx = contextFromVerifiableChunk(fcv.callbackCtx, vc, derivedBlockData)
		fcv.logger.Debug().
			Uint64("chunk_index", vc.Chunk.Index).
			Msg("Verify: system chunk contexts created, transactions will be dynamically created")
		// transactions will be dynamically created for system chunk
	} else {
		fcv.logger.Debug().
			Uint64("chunk_index", vc.Chunk.Index).
			Int("num_transactions", len(vc.ChunkDataPack.Collection.Transactions)).
			Msg("Verify: creating context and transactions for regular chunk")
		ctx = contextFromVerifiableChunk(fcv.vmCtx, vc, derivedBlockData)

		transactions = make(
			[]*fvm.TransactionProcedure,
			0,
			len(vc.ChunkDataPack.Collection.Transactions))
		for i, txBody := range vc.ChunkDataPack.Collection.Transactions {
			tx := fvm.Transaction(txBody, vc.TransactionOffset+uint32(i))
			transactions = append(transactions, tx)
		}
		fcv.logger.Debug().
			Uint64("chunk_index", vc.Chunk.Index).
			Int("num_transactions", len(transactions)).
			Msg("Verify: regular chunk transactions created")
	}

	fcv.logger.Info().
		Uint64("chunk_index", vc.Chunk.Index).
		Int("num_transactions", len(transactions)).
		Bool("is_system_chunk", vc.IsSystemChunk).
		Msg("Verify: calling verifyTransactionsInContext")

	res, err := fcv.verifyTransactionsInContext(
		ctx,
		callbackCtx,
		vc.TransactionOffset,
		vc.Chunk,
		vc.ChunkDataPack,
		vc.Result,
		transactions,
		vc.EndState,
		vc.IsSystemChunk)

	if err != nil {
		fcv.logger.Error().
			Err(err).
			Uint64("chunk_index", vc.Chunk.Index).
			Msg("Verify: verifyTransactionsInContext returned error")
	} else {
		fcv.logger.Info().
			Uint64("chunk_index", vc.Chunk.Index).
			Int("spock_secret_len", len(res)).
			Msg("Verify: verifyTransactionsInContext completed successfully")
	}

	return res, err
}

func contextFromVerifiableChunk(
	parentCtx fvm.Context,
	vc *verification.VerifiableChunkData,
	derivedBlockData *derived.DerivedBlockData,
) fvm.Context {
	return fvm.NewContextFromParent(
		parentCtx,
		fvm.WithBlockHeader(vc.Header),
		fvm.WithProtocolStateSnapshot(vc.Snapshot),
		fvm.WithDerivedBlockData(derivedBlockData),
	)
}

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

		// don't send error just return empty byte slice
		// we always assume empty value for missing registers (which might
		// cause the transaction to fail)
		// but after execution we check unknownRegTouch and if any
		// register is inside it, code won't generate approvals and
		// it activates a challenge
		return flow.RegisterValue{}, nil
	}

	return value, err
}

func (fcv *ChunkVerifier) verifyTransactionsInContext(
	context fvm.Context,
	callbackCtx fvm.Context,
	transactionOffset uint32,
	chunk *flow.Chunk,
	chunkDataPack *flow.ChunkDataPack,
	result *flow.ExecutionResult,
	transactions []*fvm.TransactionProcedure,
	endState flow.StateCommitment,
	systemChunk bool,
) (
	[]byte,
	error,
) {
	chIndex := chunk.Index
	execResID := result.ID()

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Uint32("transaction_offset", transactionOffset).
		Int("num_transactions", len(transactions)).
		Bool("system_chunk", systemChunk).
		Str("exec_result_id", execResID.String()).
		Msg("verifyTransactionsInContext: starting")

	// TODO check collection hash to match
	// TODO check datapack hash to match
	// TODO check the number of transactions and computation used

	if chunkDataPack == nil {
		return nil, fmt.Errorf("missing chunk data pack")
	}

	// Execution nodes must not include a collection for system chunks.
	if systemChunk && chunkDataPack.Collection != nil {
		return nil, chmodels.NewCFSystemChunkIncludedCollection(chIndex, execResID)
	}

	// Consensus nodes already enforce some fundamental properties of ExecutionResults:
	//   1. The result contains the correct number of chunks (compared to the block it pertains to).
	//   2. The result contains chunks with strictly monotonically increasing `Chunk.Index` starting with index 0
	//   3. for each chunk, the consistency requirement `Chunk.Index == Chunk.CollectionIndex` holds
	// See `module/validation/receiptValidator` for implementation, which is used by the consensus nodes.
	// And issue https://github.com/dapperlabs/flow-go/issues/6864 for implementing 3.
	// Hence, the following is a consistency check. Failing it means we have either encountered a critical bug,
	// or a super majority of byzantine nodes. In their case, continuing operations is impossible.
	if int(chIndex) >= len(result.Chunks) {
		return nil, chmodels.NewCFInvalidVerifiableChunk("error constructing partial trie: ",
			fmt.Errorf("chunk index out of bounds of ExecutionResult's chunk list"), chIndex, execResID)
	}

	var events flow.EventsList = nil
	serviceEvents := make(flow.ServiceEventList, 0)

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: constructing partial trie")
	// constructing a partial trie given chunk data package
	psmt, err := partial.NewLedger(chunkDataPack.Proof, ledger.State(chunkDataPack.StartState), partial.DefaultPathFinderVersion)
	if err != nil {
		// TODO provide more details based on the error type
		return nil, chmodels.NewCFInvalidVerifiableChunk(
			"error constructing partial trie: ",
			err,
			chIndex,
			execResID)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: partial trie constructed")

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: creating snapshot tree")
	// chunk view construction
	// unknown register tracks access to parts of the partial trie which
	// are not expanded and values are unknown.
	unknownRegTouch := make(map[flow.RegisterID]struct{})
	snapshotTree := snapshot.NewSnapshotTree(
		&partialLedgerStorageSnapshot{
			snapshot: executionState.NewLedgerStorageSnapshot(
				psmt,
				chunkDataPack.StartState),
			unknownRegTouch: unknownRegTouch,
		})
	chunkState := fvmState.NewExecutionState(nil, fvmState.DefaultParameters())
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: snapshot tree created")

	var problematicTx flow.Identifier
	// collect execution data formatted transaction results
	var txStartIndex int
	var processResult *flow.LightTransactionResult

	if systemChunk {
		fcv.logger.Info().
			Uint64("chunk_index", chIndex).
			Msg("verifyTransactionsInContext: creating system chunk transactions")
		transactions, processResult, err = fcv.createSystemChunk(
			callbackCtx,
			&snapshotTree,
			chunkState,
			transactionOffset,
			&events,
			&serviceEvents,
			unknownRegTouch,
			execResID,
			chIndex,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create system chunk transactions: %w", err)
		}
		fcv.logger.Info().
			Uint64("chunk_index", chIndex).
			Int("num_transactions_after_create", len(transactions)).
			Msg("verifyTransactionsInContext: system chunk transactions created")
	}

	// collect execution data formatted transaction results
	var txResults []flow.LightTransactionResult
	if len(transactions) > 0 {
		txResults = make([]flow.LightTransactionResult, len(transactions))
	}

	// If system chunk, we already executed the process callback transaction so skip it
	// by setting the start index to 1 and assigning existing process result to tx results
	if processResult != nil {
		// if process was executed, transaction length should always be at least 2 (process + system)
		txResults[0] = *processResult
		txStartIndex = 1
	}

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Int("tx_start_index", txStartIndex).
		Int("num_transactions", len(transactions)).
		Msg("verifyTransactionsInContext: starting transaction execution loop")

	// Executes all transactions in this chunk (or remaining transactions for callbacks)
	for i := txStartIndex; i < len(transactions); i++ {
		tx := transactions[i]
		ctx := context

		// For system chunks with callbacks:
		// - Process callback transaction and callback executions use callbackCtx
		// - System transaction (last one) uses the original system chunk context
		if systemChunk && context.ScheduleCallbacksEnabled && i < len(transactions)-1 {
			ctx = callbackCtx
		}

		fcv.logger.Info().
			Uint64("chunk_index", chIndex).
			Str("execution_result_id", execResID.String()).
			Int("transaction_index", i).
			Str("transaction_id", tx.ID.String()).
			Uint32("transaction_offset", transactionOffset).
			Str("procedure_type", string(tx.Type())).
			Bool("system_chunk", systemChunk).
			Bool("schedule_callbacks_enabled", context.ScheduleCallbacksEnabled).
			Msg("starting vm.Run for transaction")

		executionSnapshot, output, err := fcv.vm.Run(
			ctx,
			tx,
			snapshotTree)

		fcv.logger.Info().
			Uint64("chunk_index", chIndex).
			Str("execution_result_id", execResID.String()).
			Int("transaction_index", i).
			Str("transaction_id", tx.ID.String()).
			Uint64("computation_used", output.ComputationUsed).
			Bool("transaction_failed", output.Err != nil).
			Err(err).
			Msg("completed vm.Run for transaction")

		if err != nil {
			// this covers unexpected and very rare cases (e.g. system memory issues...),
			// so we shouldn't be here even if transaction naturally fails (e.g. permission, runtime ... )
			return nil, fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}

		fcv.logger.Info().
			Uint64("chunk_index", chIndex).
			Int("tx_index", i).
			Str("tx_id", tx.ID.String()).
			Bool("tx_failed", output.Err != nil).
			Int("num_events", len(output.Events)).
			Msg("verifyTransactionsInContext: transaction executed")

		if len(unknownRegTouch) > 0 {
			problematicTx = tx.ID
		}

		events = append(events, output.Events...)
		serviceEvents = append(serviceEvents, output.ConvertedServiceEvents...)

		snapshotTree = snapshotTree.Append(executionSnapshot)
		err = chunkState.Merge(executionSnapshot)
		if err != nil {
			return nil, fmt.Errorf("failed to merge: %d (%w)", i, err)
		}

		txResults[i] = flow.LightTransactionResult{
			TransactionID:   tx.ID,
			ComputationUsed: output.ComputationUsed,
			Failed:          output.Err != nil,
		}
	}

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Int("num_transactions_executed", len(transactions)-txStartIndex).
		Msg("verifyTransactionsInContext: transaction execution loop completed")

	// NOTE: Ignore computation usage for the purposes of comparing Cadence VM and interpreter ONLY
	for i := range txResults {
		txResults[i].ComputationUsed = 0
	}

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Int("unknown_reg_touch_count", len(unknownRegTouch)).
		Msg("verifyTransactionsInContext: checking unknown register touches")
	// check read access to unknown registers
	if len(unknownRegTouch) > 0 {
		var missingRegs []string
		for id := range unknownRegTouch {
			missingRegs = append(missingRegs, id.String())
		}
		return nil, chmodels.NewCFMissingRegisterTouch(missingRegs, chIndex, execResID, problematicTx)
	}

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Int("num_events", len(events)).
		Msg("verifyTransactionsInContext: calculating events hash")
	eventsHash, err := flow.EventsMerkleRootHash(events)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate events collection hash: %w", err)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Hex("events_hash", eventsHash[:]).
		Hex("chunk_event_collection", chunk.EventCollection[:]).
		Msg("verifyTransactionsInContext: events hash calculated")
	if chunk.EventCollection != eventsHash {
		collectionID := ""
		if chunkDataPack.Collection != nil {
			collectionID = chunkDataPack.Collection.ID().String()
		}
		for i, event := range events {
			fcv.logger.Warn().Int("list_index", i).
				Str("event_id", event.ID().String()).
				Hex("event_fingerprint", fingerprint.Fingerprint(event)).
				Str("event_type", string(event.Type)).
				Str("event_tx_id", event.TransactionID.String()).
				Uint32("event_tx_index", event.TransactionIndex).
				Uint32("event_index", event.EventIndex).
				Bytes("event_payload", event.Payload).
				Str("block_id", chunk.BlockID.String()).
				Str("collection_id", collectionID).
				Str("result_id", result.ID().String()).
				Uint64("chunk_index", chunk.Index).
				Msg("not matching events debug")
		}

		return nil, chmodels.NewCFInvalidEventsCollection(chunk.EventCollection, eventsHash, chIndex, execResID, events)
	}

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Int("num_service_events", len(serviceEvents)).
		Msg("verifyTransactionsInContext: comparing service events")
	serviceEventsInChunk := result.ServiceEventsByChunk(chunk.Index)
	equal, err := serviceEventsInChunk.EqualTo(serviceEvents)
	if err != nil {
		return nil, fmt.Errorf("error while comparing service events: %w", err)
	}
	if !equal {
		return nil, chmodels.CFInvalidServiceSystemEventsEmitted(serviceEventsInChunk, serviceEvents, chIndex, execResID)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: service events comparison completed")

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: finalizing chunk state")
	// Applying chunk updates to the partial trie.	This returns the expected
	// end state commitment after updates and the list of register keys that
	// was not provided by the chunk data package (err).
	chunkExecutionSnapshot := chunkState.Finalize()
	keys, values := executionState.RegisterEntriesToKeysValues(
		chunkExecutionSnapshot.UpdatedRegisters())
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Int("num_updated_registers", len(keys)).
		Msg("verifyTransactionsInContext: chunk state finalized")

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: creating ledger update")
	update, err := ledger.NewUpdate(
		ledger.State(chunkDataPack.StartState),
		keys,
		values)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger update: %w", err)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: ledger update created")

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: applying ledger update to partial trie")
	expEndStateComm, trieUpdate, err := psmt.Set(update)
	if err != nil {
		if errors.Is(err, ledger.ErrMissingKeys{}) {
			keys := err.(*ledger.ErrMissingKeys).Keys
			stringKeys := make([]string, len(keys))
			for i, key := range keys {
				stringKeys[i] = key.String()
			}
			return nil, chmodels.NewCFMissingRegisterTouch(stringKeys, chIndex, execResID, problematicTx)
		}
		return nil, chmodels.NewCFMissingRegisterTouch(nil, chIndex, execResID, problematicTx)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Hex("exp_end_state", expEndStateComm[:]).
		Hex("chunk_end_state", endState[:]).
		Msg("verifyTransactionsInContext: ledger update applied")

	// TODO check if exec node provided register touches that was not used (no read and no update)
	// check if the end state commitment mentioned in the chunk matches
	// what the partial trie is providing.
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: verifying end state commitment")
	if flow.StateCommitment(expEndStateComm) != endState {
		return nil, chmodels.NewCFNonMatchingFinalState(flow.StateCommitment(expEndStateComm), endState, chIndex, execResID)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: end state commitment verified")

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: starting execution data verification")
	// verify the execution data ID included in the ExecutionResult
	// 1. check basic execution data root fields
	if chunk.BlockID != chunkDataPack.ExecutionDataRoot.BlockID {
		return nil, chmodels.NewCFExecutionDataBlockIDMismatch(chunkDataPack.ExecutionDataRoot.BlockID, chunk.BlockID, chIndex, execResID)
	}

	if len(chunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs) != len(result.Chunks) {
		return nil, chmodels.NewCFExecutionDataChunksLengthMismatch(len(chunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs), len(result.Chunks), chIndex, execResID)
	}

	cedCollection := chunkDataPack.Collection
	// the system chunk collection is not included in the chunkDataPack, but is included in the
	// ChunkExecutionData. Create the collection here using the transaction bodies from the
	// transactions list (includes process callback + callback executions + system transaction)
	if systemChunk {
		fcv.logger.Info().
			Uint64("chunk_index", chIndex).
			Msg("verifyTransactionsInContext: constructing system chunk collection")
		systemTxBodies := make([]*flow.TransactionBody, len(transactions))
		for i, tx := range transactions {
			systemTxBodies[i] = tx.Transaction
		}

		cedCollection, err = flow.NewCollection(flow.UntrustedCollection{
			Transactions: systemTxBodies,
		})

		if err != nil {
			return nil, fmt.Errorf("could not construct system collection: %w", err)
		}
		fcv.logger.Info().
			Uint64("chunk_index", chIndex).
			Msg("verifyTransactionsInContext: system chunk collection constructed")
	}

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: building chunk execution data")
	// 2. build our chunk's chunk execution data using the locally calculated values, and calculate
	// its CID
	chunkExecutionData := execution_data.ChunkExecutionData{
		Collection:         cedCollection,
		Events:             events,
		TrieUpdate:         trieUpdate,
		TransactionResults: txResults,
	}

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: calculating chunk execution data CID")
	cidProvider := provider.NewExecutionDataCIDProvider(execution_data.DefaultSerializer)
	cedCID, err := cidProvider.CalculateChunkExecutionDataID(chunkExecutionData)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate CID of ChunkExecutionData: %w", err)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Str("ced_cid", cedCID.String()).
		Msg("verifyTransactionsInContext: chunk execution data CID calculated")

	// 3. check that with the chunk execution results that we created locally,
	// we can reproduce the ChunkExecutionData's ID, which the execution node is stating in its ChunkDataPack
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: verifying chunk execution data CID")
	if cedCID != chunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs[chIndex] {
		return nil, chmodels.NewCFExecutionDataInvalidChunkCID(
			chunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs[chIndex],
			cedCID,
			chIndex,
			execResID,
		)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: chunk execution data CID verified")

	// 4. check the execution data root ID by calculating it using the provided execution data root
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: calculating execution data root ID")
	executionDataID, err := cidProvider.CalculateExecutionDataRootID(chunkDataPack.ExecutionDataRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate ID of ExecutionDataRoot: %w", err)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Str("execution_data_id", executionDataID.String()).
		Str("result_execution_data_id", result.ExecutionDataID.String()).
		Msg("verifyTransactionsInContext: execution data root ID calculated")
	if executionDataID != result.ExecutionDataID {
		return nil, chmodels.NewCFInvalidExecutionDataID(result.ExecutionDataID, executionDataID, chIndex, execResID)
	}
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Msg("verifyTransactionsInContext: execution data root ID verified")

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Int("spock_secret_len", len(chunkExecutionSnapshot.SpockSecret)).
		Msg("verifyTransactionsInContext: verification completed successfully")
	return chunkExecutionSnapshot.SpockSecret, nil
}

// createSystemChunk recreates the system chunk transactions and executes the
// process callback transaction if scheduled callbacks are enabled.
//
// Returns system transaction list, which contains the system chunk transaction
// and if the scheduled callbacks are enabled it also includes process / execute
// callback transactions, and if callbacks enabled it returns the result of the
// process callback transaction. No errors are expected during normal operation.
//
// If scheduled callbacks are dissabled it will only contain the system transaction.
// If scheduled callbacks are enabled we need to do the following actions:
// 1. add and execute the process callback transaction that returns events for execute callbacks
// 2. add one transaction for each callback event
// 3. add the system transaction as last transaction
func (fcv *ChunkVerifier) createSystemChunk(
	callbackCtx fvm.Context,
	snapshotTree *snapshot.SnapshotTree,
	chunkState *fvmState.ExecutionState,
	transactionOffset uint32,
	events *flow.EventsList,
	serviceEvents *flow.ServiceEventList,
	unknownRegTouch map[flow.RegisterID]struct{},
	execResID flow.Identifier,
	chIndex uint64,
) ([]*fvm.TransactionProcedure, *flow.LightTransactionResult, error) {
	txIndex := transactionOffset

	// If scheduled callbacks are dissabled we only have the system transaction in the chunk
	if !fcv.vmCtx.ScheduleCallbacksEnabled {
		txBody, err := blueprints.SystemChunkTransaction(fcv.vmCtx.Chain)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get system chunk transaction: %w", err)
		}

		// Need to return a placeholder result that will be filled by the caller
		// when the transaction is actually executed
		return []*fvm.TransactionProcedure{
			fvm.Transaction(txBody, txIndex),
		}, nil, nil
	}

	processBody, err := blueprints.ProcessCallbacksTransaction(fcv.vmCtx.Chain)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get process callback transaction: %w", err)
	}
	processTx := fvm.Transaction(processBody, txIndex)

	// Execute process callback transaction
	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Str("execution_result_id", execResID.String()).
		Str("transaction_id", processTx.ID.String()).
		Uint32("transaction_index", txIndex).
		Str("procedure_type", string(processTx.Type())).
		Msg("starting vm.Run for process callback transaction")

	executionSnapshot, processOutput, err := fcv.vm.Run(callbackCtx, processTx, *snapshotTree)

	fcv.logger.Info().
		Uint64("chunk_index", chIndex).
		Str("execution_result_id", execResID.String()).
		Str("transaction_id", processTx.ID.String()).
		Uint64("computation_used", processOutput.ComputationUsed).
		Int("events_count", len(processOutput.Events)).
		Bool("transaction_failed", processOutput.Err != nil).
		Err(err).
		Msg("completed vm.Run for process callback transaction")

	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute process callback transaction: %w", err)
	}
	if processOutput.Err != nil {
		return nil, nil, fmt.Errorf("process callback transaction failed: %w", processOutput.Err)
	}

	processResult := &flow.LightTransactionResult{
		TransactionID:   processTx.ID,
		ComputationUsed: processOutput.ComputationUsed,
		Failed:          false,
	}

	if len(unknownRegTouch) > 0 {
		var missingRegs []string
		for id := range unknownRegTouch {
			missingRegs = append(missingRegs, id.String())
		}
		return nil, nil, chmodels.NewCFMissingRegisterTouch(missingRegs, chIndex, execResID, processTx.ID)
	}

	// Generate callback execution transactions from the events
	callbackTxs, err := blueprints.ExecuteCallbacksTransactions(fcv.vmCtx.Chain, processOutput.Events)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate callback execution transactions: %w", err)
	}

	// Build the final transaction list: [processCallback, ...callbackExecutions, systemTx]
	transactions := make([]*fvm.TransactionProcedure, 0, len(callbackTxs)+2)
	transactions = append(transactions, processTx)

	// Add callback execution transactions
	for _, c := range callbackTxs {
		txIndex++
		transactions = append(transactions, fvm.Transaction(c, txIndex))
	}

	// Add the system transaction as last transaction in collection
	systemTx, err := blueprints.SystemChunkTransaction(fcv.vmCtx.Chain)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get system chunk transaction: %w", err)
	}

	txIndex++
	transactions = append(transactions, fvm.Transaction(systemTx, txIndex))

	// Add events with pointers to reflect the change to the caller
	*events = append(*events, processOutput.Events...)
	*serviceEvents = append(*serviceEvents, processOutput.ConvertedServiceEvents...)

	*snapshotTree = snapshotTree.Append(executionSnapshot)
	err = chunkState.Merge(executionSnapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to merge process callback: %w", err)
	}

	return transactions, processResult, nil
}
