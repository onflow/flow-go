package chunks

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/verification"

	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/partial"
	chmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View, *programs.Programs) error
}

// ChunkVerifier is a verifier based on the current definitions of the flow network
type ChunkVerifier struct {
	vm             VirtualMachine
	vmCtx          fvm.Context
	systemChunkCtx fvm.Context
	logger         zerolog.Logger
}

// NewChunkVerifier creates a chunk verifier containing a flow virtual machine
func NewChunkVerifier(vm VirtualMachine, vmCtx fvm.Context, logger zerolog.Logger) *ChunkVerifier {
	return &ChunkVerifier{
		vm:    vm,
		vmCtx: vmCtx,
		systemChunkCtx: fvm.NewContextFromParent(vmCtx,
			fvm.WithRestrictedDeployment(false),
			fvm.WithTransactionFeesEnabled(false),
			fvm.WithServiceEventCollectionEnabled(),
			fvm.WithTransactionProcessors(fvm.NewTransactionInvocator(vmCtx.Logger)),
		),
		logger: logger.With().Str("component", "chunk_verifier").Logger(),
	}
}

// Verify verifies a given VerifiableChunk corresponding to a non-system chunk.
// by executing it and checking the final state commitment
// It returns a Spock Secret as a byte array, verification fault of the chunk, and an error.
// Note: Verify should only be executed on non-system chunks. It returns an error if it is invoked on
// system chunks.
func (fcv *ChunkVerifier) Verify(vc *verification.VerifiableChunkData) ([]byte, chmodels.ChunkFault, error) {
	if vc.IsSystemChunk {
		return nil, nil, fmt.Errorf("wrong method invoked for verifying system chunk")
	}

	transactions := make([]*fvm.TransactionProcedure, 0)
	for i, txBody := range vc.ChunkDataPack.Collection.Transactions {
		tx := fvm.Transaction(txBody, vc.TransactionOffset+uint32(i))
		transactions = append(transactions, tx)
	}

	return fcv.verifyTransactions(vc.Chunk, vc.ChunkDataPack, vc.Result, vc.Header, transactions, vc.EndState)
}

// SystemChunkVerify verifies a given VerifiableChunk corresponding to a system chunk.
// by executing it and checking the final state commitment
// It returns a Spock Secret as a byte array, verification fault of the chunk, and an error.
// Note: SystemChunkVerify should only be executed on system chunks. It returns an error if it is invoked on
// non-system chunks.
func (fcv *ChunkVerifier) SystemChunkVerify(vc *verification.VerifiableChunkData) ([]byte, chmodels.ChunkFault, error) {
	if !vc.IsSystemChunk {
		return nil, nil, fmt.Errorf("wrong method invoked for verifying non-system chunk")
	}

	// transaction body of system chunk
	txBody, err := blueprints.SystemChunkTransaction(fcv.vmCtx.Chain)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get system chunk transaction: %w", err)
	}

	tx := fvm.Transaction(txBody, vc.TransactionOffset+uint32(0))
	transactions := []*fvm.TransactionProcedure{tx}

	systemChunkContext := fvm.NewContextFromParent(fcv.systemChunkCtx,
		fvm.WithBlockHeader(vc.Header),
	)

	return fcv.verifyTransactionsInContext(systemChunkContext, vc.Chunk, vc.ChunkDataPack, vc.Result, transactions, vc.EndState, true)
}

func (fcv *ChunkVerifier) verifyTransactionsInContext(context fvm.Context, chunk *flow.Chunk,
	chunkDataPack *flow.ChunkDataPack,
	result *flow.ExecutionResult,
	transactions []*fvm.TransactionProcedure,
	endState flow.StateCommitment, systemChunk bool) ([]byte, chmodels.ChunkFault, error) {

	// TODO check collection hash to match
	// TODO check datapack hash to match
	// TODO check the number of transactions and computation used

	chIndex := chunk.Index
	execResID := result.ID()

	if chunkDataPack == nil {
		return nil, nil, fmt.Errorf("missing chunk data pack")
	}

	events := make(flow.EventsList, 0)
	serviceEvents := make(flow.EventsList, 0)

	// constructing a partial trie given chunk data package
	psmt, err := partial.NewLedger(chunkDataPack.Proof, ledger.State(chunkDataPack.StartState), partial.DefaultPathFinderVersion)

	if err != nil {
		// TODO provide more details based on the error type
		return nil, chmodels.NewCFInvalidVerifiableChunk("error constructing partial trie: ", err, chIndex, execResID),
			nil
	}

	// transactions in chunk can reuse the same cache, but its unknown
	// if there were changes between chunks, so we always start with a new one
	programs := programs.NewEmptyPrograms()

	// chunk view construction
	// unknown register tracks access to parts of the partial trie which
	// are not expanded and values are unknown.
	unknownRegTouch := make(map[string]*ledger.Key)
	var problematicTx flow.Identifier
	getRegister := func(owner, controller, key string) (flow.RegisterValue, error) {
		// check if register has been provided in the chunk data pack
		registerID := flow.NewRegisterID(owner, controller, key)

		registerKey := executionState.RegisterIDToKey(registerID)

		query, err := ledger.NewQuery(ledger.State(chunkDataPack.StartState), []ledger.Key{registerKey})

		if err != nil {
			return nil, fmt.Errorf("cannot create query: %w", err)
		}

		values, err := psmt.Get(query)
		if err != nil {
			if errors.Is(err, ledger.ErrMissingKeys{}) {

				unknownRegTouch[registerID.String()] = &registerKey

				// don't send error just return empty byte slice
				// we always assume empty value for missing registers (which might cause the transaction to fail)
				// but after execution we check unknownRegTouch and if any
				// register is inside it, code won't generate approvals and
				// it activates a challenge

				return []byte{}, nil
			}
			// append to missing keys if error is ErrMissingKeys

			return nil, fmt.Errorf("cannot query register: %w", err)
		}

		return values[0], nil
	}

	chunkView := delta.NewView(getRegister)

	// executes all transactions in this chunk
	for i, tx := range transactions {
		txView := chunkView.NewChild()

		err := fcv.vm.Run(context, tx, txView, programs)
		if err != nil {
			// this covers unexpected and very rare cases (e.g. system memory issues...),
			// so we shouldn't be here even if transaction naturally fails (e.g. permission, runtime ... )
			return nil, nil, fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}

		if len(unknownRegTouch) > 0 {
			problematicTx = tx.ID
		}

		events = append(events, tx.Events...)
		serviceEvents = append(serviceEvents, tx.ServiceEvents...)

		// always merge back the tx view (fvm is responsible for changes on tx errors)
		err = chunkView.MergeView(txView)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}
	}

	// check read access to unknown registers
	if len(unknownRegTouch) > 0 {
		var missingRegs []string
		for _, key := range unknownRegTouch {
			missingRegs = append(missingRegs, key.String())
		}
		return nil, chmodels.NewCFMissingRegisterTouch(missingRegs, chIndex, execResID, problematicTx), nil
	}

	eventsHash, err := flow.EventsListHash(events)
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

		computedServiceEvents := make(flow.ServiceEventList, len(serviceEvents))

		for i, serviceEvent := range serviceEvents {
			realServiceEvent, err := convert.ServiceEvent(fcv.vmCtx.Chain.ChainID(), serviceEvent)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot convert service event %d: %w", i, err)
			}
			computedServiceEvents[i] = *realServiceEvent
		}

		equal, err := result.ServiceEvents.EqualTo(computedServiceEvents)
		if err != nil {
			return nil, nil, fmt.Errorf("error while compariong service events: %w", err)
		}
		if !equal {
			return nil, chmodels.CFInvalidServiceSystemEventsEmitted(result.ServiceEvents, computedServiceEvents, chIndex, execResID), nil
		}
	}

	// applying chunk delta (register updates at chunk level) to the partial trie
	// this returns the expected end state commitment after updates and the list of
	// register keys that was not provided by the chunk data package (err).
	regs, values := chunkView.Delta().RegisterUpdates()

	update, err := ledger.NewUpdate(
		ledger.State(chunkDataPack.StartState),
		executionState.RegisterIDSToKeys(regs),
		executionState.RegisterValuesToValues(values),
	)
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

func (fcv *ChunkVerifier) verifyTransactions(chunk *flow.Chunk,
	chunkDataPack *flow.ChunkDataPack,
	result *flow.ExecutionResult,
	header *flow.Header,
	transactions []*fvm.TransactionProcedure,
	endState flow.StateCommitment) ([]byte, chmodels.ChunkFault, error) {

	// build a block context
	blockCtx := fvm.NewContextFromParent(fcv.vmCtx, fvm.WithBlockHeader(header))

	return fcv.verifyTransactionsInContext(blockCtx, chunk, chunkDataPack, result, transactions, endState, false)
}
