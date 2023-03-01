package chunks

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/verification"

	"github.com/onflow/flow-go/engine/execution/computation/computer"
	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/derived"
	fvmState "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/partial"
	chmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
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
		systemChunkCtx: computer.SystemChunkContext(vmCtx, vmCtx.Logger),
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
	chmodels.ChunkFault,
	error,
) {

	var ctx fvm.Context
	var transactions []*fvm.TransactionProcedure
	if vc.IsSystemChunk {
		ctx = fvm.NewContextFromParent(
			fcv.systemChunkCtx,
			fvm.WithBlockHeader(vc.Header))

		txBody, err := blueprints.SystemChunkTransaction(fcv.vmCtx.Chain)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get system chunk transaction: %w", err)
		}

		transactions = []*fvm.TransactionProcedure{
			fvm.Transaction(txBody, vc.TransactionOffset+uint32(0)),
		}
	} else {
		ctx = fvm.NewContextFromParent(
			fcv.vmCtx,
			fvm.WithBlockHeader(vc.Header))

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
	snapshot fvmState.StorageSnapshot

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
	chmodels.ChunkFault,
	error,
) {

	// TODO check collection hash to match
	// TODO check datapack hash to match
	// TODO check the number of transactions and computation used

	chIndex := chunk.Index
	execResID := result.ID()

	if chunkDataPack == nil {
		return nil, nil, fmt.Errorf("missing chunk data pack")
	}

	events := make(flow.EventsList, 0)
	serviceEvents := make(flow.ServiceEventList, 0)

	// constructing a partial trie given chunk data package
	psmt, err := partial.NewLedger(chunkDataPack.Proof, ledger.State(chunkDataPack.StartState), partial.DefaultPathFinderVersion)

	if err != nil {
		// TODO provide more details based on the error type
		return nil, chmodels.NewCFInvalidVerifiableChunk(
				"error constructing partial trie: ",
				err,
				chIndex,
				execResID),
			nil
	}

	context = fvm.NewContextFromParent(
		context,
		fvm.WithDerivedBlockData(
			derived.NewEmptyDerivedBlockDataWithTransactionOffset(
				transactionOffset)))

	// chunk view construction
	// unknown register tracks access to parts of the partial trie which
	// are not expanded and values are unknown.
	unknownRegTouch := make(map[flow.RegisterID]struct{})
	chunkView := delta.NewDeltaView(
		&partialLedgerStorageSnapshot{
			snapshot: executionState.NewLedgerStorageSnapshot(
				psmt,
				chunkDataPack.StartState),
			unknownRegTouch: unknownRegTouch,
		})

	var problematicTx flow.Identifier
	// executes all transactions in this chunk
	for i, tx := range transactions {
		txView := chunkView.NewChild()

		err := fcv.vm.Run(context, tx, txView)
		if err != nil {
			// this covers unexpected and very rare cases (e.g. system memory issues...),
			// so we shouldn't be here even if transaction naturally fails (e.g. permission, runtime ... )
			return nil, nil, fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}

		if len(unknownRegTouch) > 0 {
			problematicTx = tx.ID
		}

		events = append(events, tx.Events...)
		serviceEvents = append(serviceEvents, tx.ConvertedServiceEvents...)

		// always merge back the tx view (fvm is responsible for changes on tx errors)
		err = chunkView.MergeView(txView)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}
	}

	// check read access to unknown registers
	if len(unknownRegTouch) > 0 {
		var missingRegs []string
		for id := range unknownRegTouch {
			missingRegs = append(missingRegs, id.String())
		}
		return nil, chmodels.NewCFMissingRegisterTouch(missingRegs, chIndex, execResID, problematicTx), nil
	}

	eventsHash, err := flow.EventsMerkleRootHash(events)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot calculate events collection hash: %w", err)
	}
	if chunk.EventCollection != eventsHash {

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
				Str("collection_id", chunkDataPack.Collection.ID().String()).
				Str("result_id", result.ID().String()).
				Uint64("chunk_index", chunk.Index).
				Msg("not matching events debug")
		}

		return nil, chmodels.NewCFInvalidEventsCollection(chunk.EventCollection, eventsHash, chIndex, execResID, events), nil
	}

	if systemChunk {
		equal, err := result.ServiceEvents.EqualTo(serviceEvents)
		if err != nil {
			return nil, nil, fmt.Errorf("error while comparing service events: %w", err)
		}
		if !equal {
			return nil, chmodels.CFInvalidServiceSystemEventsEmitted(result.ServiceEvents, serviceEvents, chIndex, execResID), nil
		}
	}

	// applying chunk delta (register updates at chunk level) to the partial trie
	// this returns the expected end state commitment after updates and the list of
	// register keys that was not provided by the chunk data package (err).
	keys, values := executionState.RegisterEntriesToKeysValues(
		chunkView.Delta().UpdatedRegisters())

	update, err := ledger.NewUpdate(
		ledger.State(chunkDataPack.StartState),
		keys,
		values)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

	expEndStateComm, _, err := psmt.Set(update)

	if err != nil {
		if errors.Is(err, ledger.ErrMissingKeys{}) {
			keys := err.(*ledger.ErrMissingKeys).Keys
			stringKeys := make([]string, len(keys))
			for i, key := range keys {
				stringKeys[i] = key.String()
			}
			return nil, chmodels.NewCFMissingRegisterTouch(stringKeys, chIndex, execResID, problematicTx), nil
		}
		return nil, chmodels.NewCFMissingRegisterTouch(nil, chIndex, execResID, problematicTx), nil
	}

	// TODO check if exec node provided register touches that was not used (no read and no update)
	// check if the end state commitment mentioned in the chunk matches
	// what the partial trie is providing.
	if flow.StateCommitment(expEndStateComm) != endState {
		return nil, chmodels.NewCFNonMatchingFinalState(flow.StateCommitment(expEndStateComm), endState, chIndex, execResID), nil
	}
	return chunkView.SpockSecret(), nil, nil
}
