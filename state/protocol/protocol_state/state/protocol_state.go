package state

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolState is an implementation of the read-only interface for protocol state, it allows querying information
// on a per-block and per-epoch basis.
// It is backed by a storage.EpochProtocolStateEntries and an in-memory protocol.GlobalParams.
type ProtocolState struct {
	epochProtocolStateDB storage.EpochProtocolStateEntries
	kvStoreSnapshots     protocol_state.ProtocolKVStore
	globalParams         protocol.GlobalParams
}

var _ protocol.ProtocolState = (*ProtocolState)(nil)

func NewProtocolState(epochProtocolStateDB storage.EpochProtocolStateEntries, kvStoreSnapshots storage.ProtocolKVStore, globalParams protocol.GlobalParams) *ProtocolState {
	return newProtocolState(epochProtocolStateDB, kvstore.NewProtocolKVStore(kvStoreSnapshots), globalParams)
}

// newProtocolState creates a new ProtocolState instance. The exported constructor `NewProtocolState` only requires the
// lower-level `storage.ProtocolKVStore` as input. Though, internally we use the higher-level `protocol_state.ProtocolKVStore`,
// which wraps the lower-level ProtocolKVStore.
func newProtocolState(epochProtocolStateDB storage.EpochProtocolStateEntries, kvStoreSnapshots protocol_state.ProtocolKVStore, globalParams protocol.GlobalParams) *ProtocolState {
	return &ProtocolState{
		epochProtocolStateDB: epochProtocolStateDB,
		kvStoreSnapshots:     kvStoreSnapshots,
		globalParams:         globalParams,
	}
}

// EpochStateAtBlockID returns epoch protocol state at block ID.
// The resulting epoch protocol state is returned AFTER applying updates that are contained in block.
// Can be queried for any block that has been added to the block tree.
// Returns:
// - (EpochProtocolState, nil) - if there is an epoch protocol state associated with given block ID.
// - (nil, storage.ErrNotFound) - if there is no epoch protocol state associated with given block ID.
// - (nil, exception) - any other error should be treated as exception.
func (s *ProtocolState) EpochStateAtBlockID(blockID flow.Identifier) (protocol.EpochProtocolState, error) {
	protocolStateEntry, err := s.epochProtocolStateDB.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not query epoch protocol state at block (%x): %w", blockID, err)
	}
	return inmem.NewEpochProtocolStateAdapter(protocolStateEntry, s.globalParams), nil
}

// KVStoreAtBlockID returns protocol state at block ID.
// The resulting protocol state is returned AFTER applying updates that are contained in block.
// Can be queried for any block that has been added to the block tree.
// Returns:
// - (KVStoreReader, nil) - if there is a protocol state associated with given block ID.
// - (nil, storage.ErrNotFound) - if there is no protocol state associated with given block ID.
// - (nil, exception) - any other error should be treated as exception.
func (s *ProtocolState) KVStoreAtBlockID(blockID flow.Identifier) (protocol.KVStoreReader, error) {
	return s.kvStoreSnapshots.ByBlockID(blockID)
}

// GlobalParams returns an interface which can be used to query global protocol parameters.
func (s *ProtocolState) GlobalParams() protocol.GlobalParams {
	return s.globalParams
}

// MutableProtocolState is an implementation of the mutable interface for protocol state, it allows to evolve the protocol state
// by acting as factory for protocol.StateMutator which can be used to apply state-changing operations.
type MutableProtocolState struct {
	ProtocolState
	headers                 storage.Headers
	results                 storage.ExecutionResults
	kvStateMachineFactories []protocol_state.KeyValueStoreStateMachineFactory
}

var _ protocol.MutableProtocolState = (*MutableProtocolState)(nil)

// NewMutableProtocolState creates a new instance of MutableProtocolState.
func NewMutableProtocolState(
	epochProtocolStateDB storage.EpochProtocolStateEntries,
	kvStoreSnapshots storage.ProtocolKVStore,
	globalParams protocol.GlobalParams,
	headers storage.Headers,
	results storage.ExecutionResults,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
) *MutableProtocolState {
	// an ordered list of factories to create state machines for different sub-states of the Dynamic Protocol State.
	// all factories are expected to be called in order defined here.
	kvStateMachineFactories := []protocol_state.KeyValueStoreStateMachineFactory{
		kvstore.NewPSVersionUpgradeStateMachineFactory(globalParams),
		epochs.NewEpochStateMachineFactory(globalParams, setups, commits, epochProtocolStateDB),
	}
	return newMutableProtocolState(epochProtocolStateDB, kvstore.NewProtocolKVStore(kvStoreSnapshots), globalParams, headers, results, kvStateMachineFactories)
}

// newMutableProtocolState creates a new instance of MutableProtocolState, where we inject factories for the orthogonal
// state machines evolving the sub-states. This constructor should be used mainly for testing (hence it is not exported).
// Specifically, the MutableProtocolState is conceptually independent of the specific functions that the state machines
// implement. Therefore, we test it independently of the state machines required for production. In comparison, the
// constructor `NewMutableProtocolState` is intended for production use, where the list of state machines is hard-coded.
func newMutableProtocolState(
	epochProtocolStateDB storage.EpochProtocolStateEntries,
	kvStoreSnapshots protocol_state.ProtocolKVStore,
	globalParams protocol.GlobalParams,
	headers storage.Headers,
	results storage.ExecutionResults,
	kvStateMachineFactories []protocol_state.KeyValueStoreStateMachineFactory,
) *MutableProtocolState {
	return &MutableProtocolState{
		ProtocolState:           *newProtocolState(epochProtocolStateDB, kvStoreSnapshots, globalParams),
		headers:                 headers,
		results:                 results,
		kvStateMachineFactories: kvStateMachineFactories,
	}
}

// EvolveState updates the overall Protocol State based on information in the candidate block
// (potentially still under construction). Information that may change the state is:
//   - the candidate block's view
//   - Service Events from execution results sealed in the candidate block
//
// EvolveState is compatible with speculative processing: it evolves an *in-memory copy* of the parent state
// and collects *deferred database updates* for persisting the resulting Protocol State, including all of its
// dependencies and respective indices. Though, the resulting batch of deferred database updates still depends
// on the candidate block's ID, which is still unknown at the time of block construction. Executing the deferred
// database updates is the caller's responsibility.
//
// SAFETY REQUIREMENTS:
//  1. The seals must be a protocol-compliant extension of the parent block. Intuitively, we require that the
//     seals follow the ancestry of this fork without gaps. The Consensus Participant's Compliance Layer enforces
//     the necessary constrains. Analogously, the block building logic should always produce protocol-compliant
//     seals.
//     The seals guarantee correctness of the sealed execution result, including the contained service events.
//     This is actively checked by the verification node, whose aggregated approvals in the form of a seal attest
//     to the correctness of the sealed execution result (specifically the Service Events contained in the result
//     and their order).
//  2. For Consensus Participants that are replicas, the calling code must check that the returned `stateID` matches
//     the commitment in the block proposal! If they don't match, the proposal is byzantine and should be slashed.
//
// Error returns:
// [TLDR] All error returns indicate potential state corruption and should therefore be treated as fatal.
//   - Per convention, the input seals from the block payload have already been confirmed to be protocol compliant.
//     Hence, the service events in the sealed execution results represent the honest execution path.
//     Therefore, the sealed service events should encode a valid evolution of the protocol state -- provided
//     the system smart contracts are correct.
//   - As we can rule out byzantine attacks as the source of failures, the only remaining sources of problems
//     can be (a) bugs in the system smart contracts or (b) bugs in the node implementation. A service event
//     not representing a valid state transition despite all consistency checks passing is interpreted as
//     case (a) and _should be_ handled internally by the respective state machine. Otherwise, any bug or
//     unforeseen edge cases in the system smart contracts would result in consensus halt, due to errors while
//     evolving the protocol state.
//   - A consistency or sanity check failing within the StateMutator is likely the symptom of an internal bug
//     in the node software or state corruption, i.e. case (b). This is the only scenario where the error return
//     of this function is not nil. If such an exception is returned, continuing is not an option.
func (s *MutableProtocolState) EvolveState(
	parentBlockID flow.Identifier,
	candidateView uint64,
	candidateSeals []*flow.Seal,
) (flow.Identifier, *transaction.DeferredBlockPersist, error) {
	serviceEvents, err := s.serviceEventsFromSeals(candidateSeals)
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("extracting service events from candidate seals failed: %w", err)
	}

	parentStateID, stateMachines, evolvingState, err := s.initializeOrthogonalStateMachines(parentBlockID, candidateView)
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("failure initializing sub-state machines for evolving the Protocol State: %w", err)
	}

	resultingStateID, dbUpdates, err := s.build(parentStateID, stateMachines, serviceEvents, evolvingState)
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("evolving and building the resulting Protocol State failed: %w", err)
	}
	return resultingStateID, dbUpdates, nil
}

// initializeOrthogonalStateMachines instantiates the sub-state machines that in aggregate evolve the protocol state.
// In a nutshell, we proceed as follows:
//  1. We retrieve the protocol state snapshot that the parent block committed to.
//  2. We determine which Protocol State version should be active at the candidate block's view. Note that there might be a
//     pending Version Upgrade that was supposed to take effect at an earlier view `activationView` < `candidateView`. However,
//     it is possible that there are no blocks in the current fork with views in [activationView, …, candidateView]. In this
//     case, the version upgrade is still pending and should be activated now, despite its activationView having already passed.
//  3. We replicate the parent block Protocol State -- if necessary changing the data model in accordance with the Protocol
//     State version that should be active at the candidate block's view. Essentially, this replication is a deep copy, which
//     guarantees that we are not accidentally modifying the parent block's protocol state.
//  4. Initialize the sub-state machines via the injected factories and provide the *replicated* state as an in-memory target
//     for the state machines to write their evolved sub-states to.
func (s *MutableProtocolState) initializeOrthogonalStateMachines(
	parentBlockID flow.Identifier,
	candidateView uint64,
) (flow.Identifier, []protocol_state.KeyValueStoreStateMachine, protocol_state.KVStoreMutator, error) {
	parentState, err := s.kvStoreSnapshots.ByBlockID(parentBlockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return flow.ZeroID, nil, nil, irrecoverable.NewExceptionf("Protocol State at parent block %v was not found: %w", parentBlockID, err)
		}
		return flow.ZeroID, nil, nil, fmt.Errorf("unexpected exception while retrieving Protocol State at parent block %v: %w", parentBlockID, err)
	}

	protocolVersion := parentState.GetProtocolStateVersion()
	if versionUpgrade := parentState.GetVersionUpgrade(); versionUpgrade != nil {
		if candidateView >= versionUpgrade.ActivationView {
			protocolVersion = versionUpgrade.Data
		}
	}

	evolvingState, err := parentState.Replicate(protocolVersion)
	if err != nil {
		if errors.Is(err, kvstore.ErrIncompatibleVersionChange) {
			return flow.ZeroID, nil, nil, irrecoverable.NewExceptionf("replicating parent block's protocol state failed due to unsupported version: %w", err)
		}
		return flow.ZeroID, nil, nil, fmt.Errorf("could not replicate parent KV store (version=%d) to protocol version %d: %w", parentState.GetProtocolStateVersion(), protocolVersion, err)
	}

	stateMachines := make([]protocol_state.KeyValueStoreStateMachine, 0, len(s.kvStateMachineFactories))
	for _, factory := range s.kvStateMachineFactories {
		stateMachine, err := factory.Create(candidateView, parentBlockID, parentState, evolvingState)
		if err != nil {
			return flow.ZeroID, nil, nil, fmt.Errorf("could not create state machine: %w", err)
		}
		stateMachines = append(stateMachines, stateMachine)
	}
	return parentState.ID(), stateMachines, evolvingState, nil
}

// serviceEventsFromSeals arranges the sealed results in order of increasing height of the executed blocks
// and then extracts all Service Events from the sealed results. While the seals might be included in the
// candidate block in any order, _within_ a sealed execution result, the service events are chronologically
// ordered. Hence, by arranging the seals by increasing height, the total order of all extracted Service
// Events is also chronological.
func (s *MutableProtocolState) serviceEventsFromSeals(candidateSeals []*flow.Seal) ([]flow.ServiceEvent, error) {
	// block payload may not specify seals in order, so order them by block height before processing
	orderedSeals, err := protocol.OrderedSeals(candidateSeals, s.headers)
	if err != nil {
		// Per API contract, the input seals must have already passed verification, which necessitates
		// successful ordering. Hence, calling protocol.OrderedSeals with the same inputs that succeeded
		// earlier now failed. In all cases, this is an exception.
		if errors.Is(err, protocol.ErrMultipleSealsForSameHeight) || errors.Is(err, protocol.ErrDiscontinuousSeals) || errors.Is(err, storage.ErrNotFound) {
			return nil, irrecoverable.NewExceptionf("ordering already validated seals unexpectedly failed: %w", err)
		}
		return nil, fmt.Errorf("ordering already validated seals resulted in unexpected exception: %w", err)
	}

	serviceEvents := make([]flow.ServiceEvent, 0) // we expect that service events are rare; most blocks have none
	for _, seal := range orderedSeals {
		result, err := s.results.ByID(seal.ResultID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, irrecoverable.NewExceptionf("could not get result %x sealed by valid seal %x: %w", seal.ResultID, seal.ID(), err)
			}
			return nil, fmt.Errorf("retrieving result %x resulted in unexpected exception: %w", seal.ResultID, err)
		}
		serviceEvents = append(serviceEvents, result.ServiceEvents...)
	}
	return serviceEvents, nil
}

// build assembled the final Protocol State.
// First, we apply the service events to all sub-state machines and then build the resulting state.
// Thereby, the framework supports a subtly more general way of partitioning the Protocol State machine,
// where state machines could exchange some information, if their chronological order of execution is strictly
// specified and guaranteed. The framework conceptually tolerates this, without explicitly supporting it (yet).
//
// Returns:
//   - ID of the resulting Protocol State
//   - deferred database operations for persisting the resulting Protocol State, including all of its
//     dependencies and respective indices. Though, the resulting batch of deferred database updates still depends
//     on the candidate block's ID, which is still unknown at the time of block construction.
//   - err: All error returns indicate potential state corruption and should therefore be treated as fatal.
func (s *MutableProtocolState) build(
	parentStateID flow.Identifier,
	stateMachines []protocol_state.KeyValueStoreStateMachine,
	serviceEvents []flow.ServiceEvent,
	evolvingState protocol_state.KVStoreMutator,
) (flow.Identifier, *transaction.DeferredBlockPersist, error) {
	for _, stateMachine := range stateMachines {
		err := stateMachine.EvolveState(serviceEvents) // state machine should only bubble up exceptions
		if err != nil {
			return flow.ZeroID, nil, fmt.Errorf("exception from sub-state machine during state evolution: %w", err)
		}
	}

	// _after_ all state machines have ingested the available information, we build the resulting overall state
	dbUpdates := transaction.NewDeferredBlockPersist()
	for _, stateMachine := range stateMachines {
		dbOps, err := stateMachine.Build()
		if err != nil {
			return flow.ZeroID, nil, fmt.Errorf("unexpected exception from sub-state machine while building its output state: %w", err)
		}
		dbUpdates.AddIndexingOps(dbOps.Pending())
	}
	resultingStateID := evolvingState.ID()

	// We _always_ index the protocol state by the candidate block's ID. But only if the
	// state actually changed, we add a database operation to persist it.
	dbUpdates.AddIndexingOp(func(blockID flow.Identifier, tx *transaction.Tx) error {
		return s.kvStoreSnapshots.IndexTx(blockID, resultingStateID)(tx)
	})
	if parentStateID != resultingStateID {
		// note that `SkipDuplicatesTx` is still required, because the result might equal to an earlier known state (we explicitly want to de-duplicate)
		dbUpdates.AddDbOp(operation.SkipDuplicatesTx(s.kvStoreSnapshots.StoreTx(resultingStateID, evolvingState)))
	}

	return resultingStateID, dbUpdates, nil
}
