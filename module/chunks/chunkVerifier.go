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
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
)

// ChunkVerifier is a verifier based on the current definitions of the flow network
type ChunkVerifier struct {
	vm             fvm.VM
	vmCtx          fvm.Context
	systemChunkCtx fvm.Context
	logger         zerolog.Logger
}

// NewChunkVerifier creates a chunk verifier containing a flow virtual machine
func NewChunkVerifier(vm fvm.VM, vmCtx fvm.Context, logger zerolog.Logger) *ChunkVerifier {
	return &ChunkVerifier{
		vm:             vm,
		vmCtx:          vmCtx,
		systemChunkCtx: computer.SystemChunkContext(vmCtx),
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

	var ctx fvm.Context
	var transactions []*fvm.TransactionProcedure
	if vc.IsSystemChunk {
		ctx = fvm.NewContextFromParent(
			fcv.systemChunkCtx,
			fvm.WithBlockHeader(vc.Header),
			// `protocol.Snapshot` implements `EntropyProvider` interface
			// Note that `Snapshot` possible errors for RandomSource() are:
			// - storage.ErrNotFound if the QC is unknown.
			// - state.ErrUnknownSnapshotReference if the snapshot reference block is unknown
			// However, at this stage, snapshot reference block should be known and the QC should also be known,
			// so no error is expected in normal operations, as required by `EntropyProvider`.
			fvm.WithEntropyProvider(vc.Snapshot),
		)

		txBody, err := blueprints.SystemChunkTransaction(fcv.vmCtx.Chain)
		if err != nil {
			return nil, fmt.Errorf("could not get system chunk transaction: %w", err)
		}

		transactions = []*fvm.TransactionProcedure{
			fvm.Transaction(txBody, vc.TransactionOffset+uint32(0)),
		}
	} else {
		ctx = fvm.NewContextFromParent(
			fcv.vmCtx,
			fvm.WithBlockHeader(vc.Header),
			// `protocol.Snapshot` implements `EntropyProvider` interface
			// Note that `Snapshot` possible errors for RandomSource() are:
			// - storage.ErrNotFound if the QC is unknown.
			// - state.ErrUnknownSnapshotReference if the snapshot reference block is unknown
			// However, at this stage, snapshot reference block should be known and the QC should also be known,
			// so no error is expected in normal operations, as required by `EntropyProvider`.
			fvm.WithEntropyProvider(vc.Snapshot),
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

	return fcv.verifyTransactionsInContext(
		ctx,
		vc.TransactionOffset,
		vc.Chunk,
		vc.ChunkDataPack,
		vc.Result,
		transactions,
		vc.EndState,
		vc.IsSystemChunk)
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

	// TODO check collection hash to match
	// TODO check datapack hash to match
	// TODO check the number of transactions and computation used

	chIndex := chunk.Index
	execResID := result.ID()

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

	context = fvm.NewContextFromParent(
		context,
		fvm.WithDerivedBlockData(
			derived.NewEmptyDerivedBlockData(logical.Time(transactionOffset))))

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

	var problematicTx flow.Identifier

	// collect execution data formatted transaction results
	var txResults []flow.LightTransactionResult
	if len(transactions) > 0 {
		txResults = make([]flow.LightTransactionResult, len(transactions))
	}

	// executes all transactions in this chunk
	for i, tx := range transactions {
		executionSnapshot, output, err := fcv.vm.Run(
			context,
			tx,
			snapshotTree)
		if err != nil {
			// this covers unexpected and very rare cases (e.g. system memory issues...),
			// so we shouldn't be here even if transaction naturally fails (e.g. permission, runtime ... )
			return nil, fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}

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

	// check read access to unknown registers
	if len(unknownRegTouch) > 0 {
		var missingRegs []string
		for id := range unknownRegTouch {
			missingRegs = append(missingRegs, id.String())
		}
		return nil, chmodels.NewCFMissingRegisterTouch(missingRegs, chIndex, execResID, problematicTx)
	}

	eventsHash, err := flow.EventsMerkleRootHash(events)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate events collection hash: %w", err)
	}
	if chunk.EventCollection != eventsHash {
		collectionID := ""
		if chunkDataPack.Collection != nil {
			collectionID = chunkDataPack.Collection.ID().String()
		}
		for i, event := range events {
			fcv.logger.Warn().Int("list_index", i).
				Str("event_id", event.ID().String()).
				Hex("event_fingerptint", event.Fingerprint()).
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

	if systemChunk {
		equal, err := result.ServiceEvents.EqualTo(serviceEvents)
		if err != nil {
			return nil, fmt.Errorf("error while comparing service events: %w", err)
		}
		if !equal {
			return nil, chmodels.CFInvalidServiceSystemEventsEmitted(result.ServiceEvents, serviceEvents, chIndex, execResID)
		}
	}

	// Applying chunk updates to the partial trie.	This returns the expected
	// end state commitment after updates and the list of register keys that
	// was not provided by the chunk data package (err).
	chunkExecutionSnapshot := chunkState.Finalize()
	keys, values := executionState.RegisterEntriesToKeysValues(
		chunkExecutionSnapshot.UpdatedRegisters())

	update, err := ledger.NewUpdate(
		ledger.State(chunkDataPack.StartState),
		keys,
		values)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

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

	// TODO check if exec node provided register touches that was not used (no read and no update)
	// check if the end state commitment mentioned in the chunk matches
	// what the partial trie is providing.
	if flow.StateCommitment(expEndStateComm) != endState {
		return nil, chmodels.NewCFNonMatchingFinalState(flow.StateCommitment(expEndStateComm), endState, chIndex, execResID)
	}

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
	// ChunkExecutionData. Create the collection here using the transaction body from the
	// transactions list
	if systemChunk {
		cedCollection = &flow.Collection{
			Transactions: []*flow.TransactionBody{transactions[0].Transaction},
		}
	}

	// 2. build our chunk's chunk execution data using the locally calculated values, and calculate
	// its CID
	chunkExecutionData := execution_data.ChunkExecutionData{
		Collection:         cedCollection,
		Events:             events,
		TrieUpdate:         trieUpdate,
		TransactionResults: txResults,
	}

	cidProvider := provider.NewExecutionDataCIDProvider(execution_data.DefaultSerializer)
	cedCID, err := cidProvider.CalculateChunkExecutionDataID(chunkExecutionData)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate CID of ChunkExecutionData: %w", err)
	}

	// 3. check that with the chunk execution results that we created locally,
	// we can reproduce the ChunkExecutionData's ID, which the execution node is stating in its ChunkDataPack
	if cedCID != chunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs[chIndex] {
		return nil, chmodels.NewCFExecutionDataInvalidChunkCID(
			chunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs[chIndex],
			cedCID,
			chIndex,
			execResID,
		)
	}

	// 4. check the execution data root ID by calculating it using the provided execution data root
	executionDataID, err := cidProvider.CalculateExecutionDataRootID(chunkDataPack.ExecutionDataRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate ID of ExecutionDataRoot: %w", err)
	}
	if executionDataID != result.ExecutionDataID {
		return nil, chmodels.NewCFInvalidExecutionDataID(result.ExecutionDataID, executionDataID, chIndex, execResID)
	}

	return chunkExecutionSnapshot.SpockSecret, nil
}
