package state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// StateMachineFactoryMethod is a factory to create state machines for evolving the protocol state.
// Currently, we have `protocolStateMachine` and `epochFallbackStateMachine` as StateMachine
// implementations, whose constructors both have the same signature as StateMachineFactoryMethod.
type StateMachineFactoryMethod = func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.StateMachine, error)

// stateMutator is a stateful object to evolve the protocol state. It is instantiated from the parent block's protocol state.
// State-changing operations can be iteratively applied and the stateMutator will internally evolve its in-memory state.
// While the StateMutator does not modify the database, it internally tracks the necessary database updates to persist its
// dependencies (specifically EpochSetup and EpochCommit events). Upon calling `Build` the stateMutator returns the updated
// protocol state, its ID and all database updates necessary for persisting the updated protocol state.
//
// The StateMutator is used by a replica's compliance layer to update protocol state when observing state-changing service in
// blocks. It is used by the primary in the block building process to obtain the correct protocol state for a proposal.
// Specifically, the leader may include state-changing service events in the block payload. The flow protocol prescribes that
// the proposal needs to include the ID of the protocol state, _after_ processing the payload incl. all state-changing events.
// Therefore, the leader instantiates a StateMutator, applies the service events to it and builds the updated protocol state ID.
//
// Not safe for concurrent use.
type stateMutator struct {
	headers          storage.Headers
	results          storage.ExecutionResults
	kvStoreSnapshots storage.ProtocolKVStore

	candidate            *flow.Header
	parentState          protocol_state.KVStoreReader
	kvMutator            protocol_state.KVStoreMutator
	orthoKVStoreMachines []protocol_state.KeyValueStoreStateMachine
}

var _ protocol.StateMutator = (*stateMutator)(nil)

// newStateMutator creates a new instance of stateMutator.
// stateMutator performs initialization of state machine depending on the operation mode of the protocol.
// No errors are expected during normal operations.
func newStateMutator(
	headers storage.Headers,
	results storage.ExecutionResults,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	protocolStateSnapshots storage.ProtocolState,
	kvStoreSnapshots storage.ProtocolKVStore,
	params protocol.GlobalParams,
	candidate *flow.Header,
	parentState protocol_state.KVStoreAPI,
) (*stateMutator, error) {
	stateMachines := make([]protocol_state.KeyValueStoreStateMachine, 0)

	protocolVersion := parentState.GetProtocolStateVersion()
	if versionUpgrade := parentState.GetVersionUpgrade(); versionUpgrade != nil {
		if candidate.View >= versionUpgrade.ActivationView {
			protocolVersion = versionUpgrade.Data
		}
	}

	replicatedState, err := parentState.Replicate(protocolVersion)
	if err != nil {
		return nil, fmt.Errorf("could not replicate parent KV store (version=%d) to protocol version %d: %w",
			parentState.GetProtocolStateVersion(), protocolVersion, err)
	}

	epochsStateMachine, err := epochs.NewEpochStateMachine(
		candidate,
		params,
		setups,
		commits,
		protocolStateSnapshots,
		parentState,
		replicatedState,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create epoch state machine: %w", err)
	}

	versionUpgradeStateMachine := kvstore.NewPSVersionUpgradeStateMachine(candidate.View, params, parentState, replicatedState)

	// at this point, we define the order in which state machines process updates
	stateMachines = append(stateMachines, epochsStateMachine)
	stateMachines = append(stateMachines, versionUpgradeStateMachine)

	return &stateMutator{
		candidate:            candidate,
		headers:              headers,
		results:              results,
		kvStoreSnapshots:     kvStoreSnapshots,
		orthoKVStoreMachines: stateMachines,
		parentState:          parentState,
		kvMutator:            replicatedState,
	}, nil
}

// Build returns:
//   - hasChanges: flag whether there were any changes; otherwise, `updatedState` and `stateID` equal the parent state
//   - updatedState: the ProtocolState after applying all updates.
//   - stateID: the hash commitment to the `updatedState`
//   - dbUpdates: database updates necessary for persisting the updated protocol state's *dependencies*.
//     If hasChanges is false, updatedState is empty. Caution: persisting the `updatedState` itself and adding
//     it to the relevant indices is _not_ in `dbUpdates`. Persisting and indexing `updatedState` is the responsibility
//     of the calling code (specifically `FollowerState`).
//
// updated protocol state entry, state ID and a flag indicating if there were any changes.
func (m *stateMutator) Build() (flow.Identifier, []transaction.DeferredDBUpdate, error) {
	var dbUpdates []transaction.DeferredDBUpdate
	for _, stateMachine := range m.orthoKVStoreMachines {
		dbUpdates = append(dbUpdates, stateMachine.Build()...)
	}
	stateID := m.kvMutator.ID()

	// Schedule deferred database operations to index the protocol state by the candidate block's ID
	// and persist the new protocol state (if there are any changes)
	dbUpdates = append(dbUpdates, m.kvStoreSnapshots.IndexTx(m.candidate.ID(), stateID))
	version, data, err := m.kvMutator.VersionedEncode()
	if err != nil {
		return flow.ZeroID, nil, fmt.Errorf("could not encode protocol state: %w", err)
	}
	dbUpdates = append(dbUpdates, operation.SkipDuplicatesTx(m.kvStoreSnapshots.StoreTx(stateID, &storage.KeyValueStoreData{
		Version: version,
		Data:    data,
	})))

	return stateID, dbUpdates, nil
}

// ApplyServiceEventsFromValidatedSeals applies the state changes that are delivered via
// sealed service events:
//   - iterating over the sealed service events in order of increasing height
//   - identifying state-changing service event and calling into the embedded
//     StateMachine to apply the respective state update
//   - tracking deferred database updates necessary to persist the updated
//     protocol state's *dependencies*. Persisting and indexing `updatedState`
//     is the responsibility of the calling code (specifically `FollowerState`)
//
// All updates only mutate the `StateMutator`'s internal in-memory copy of the
// protocol state, without changing the parent state (i.e. the state we started from).
//
// SAFETY REQUIREMENT:
// The StateMutator assumes that the proposal has passed the following correctness checks!
//   - The seals in the payload continuously follow the ancestry of this fork. Specifically,
//     there are no gaps in the seals.
//   - The seals guarantee correctness of the sealed execution result, including the contained
//     service events. This is actively checked by the verification node, whose aggregated
//     approvals in the form of a seal attest to the correctness of the sealed execution result,
//     including the contained.
//
// Consensus nodes actively verify protocol compliance for any block proposal they receive,
// including integrity of each seal individually as well as the seals continuously following the
// fork. Light clients only process certified blocks, which guarantees that consensus nodes already
// ran those checks and found the proposal to be valid.
//
// Details on SERVICE EVENTS:
// Consider a chain where a service event is emitted during execution of block A.
// Block B contains an execution receipt for A. Block C contains a seal for block
// A's execution result.
//
//	A <- .. <- B(RA) <- .. <- C(SA)
//
// Service Events are included within execution results, which are stored
// opaquely as part of the block payload in block B. We only validate, process and persist
// the typed service event to storage once we process C, the block containing the
// seal for block A. This is because we rely on the sealing subsystem to validate
// correctness of the service event before processing it.
// Consequently, any change to the protocol state introduced by a service event
// emitted during execution of block A would only become visible when querying
// C or its descendants.
//
// Error returns:
//   - Per convention, the input seals from the block payload have already been confirmed to be protocol compliant.
//     Hence, the service events in the sealed execution results represent the honest execution path.
//     Therefore, the sealed service events should encode a valid evolution of the protocol state -- provided
//     the system smart contracts are correct.
//   - As we can rule out byzantine attacks as the source of failures, the only remaining sources of problems
//     can be (a) bugs in the system smart contracts or (b) bugs in the node implementation.
//     A service event not representing a valid state transition despite all consistency checks passing
//     is interpreted as case (a) and handled internally within the StateMutator. In short, we go into Epoch
//     Fallback Mode by copying the parent state (a valid state snapshot) and setting the
//     `InvalidEpochTransitionAttempted` flag. All subsequent Epoch-lifecycle events are ignored.
//   - A consistency or sanity check failing within the StateMutator is likely the symptom of an internal bug
//     in the node software or state corruption, i.e. case (b). This is the only scenario where the error return
//     of this function is not nil. If such an exception is returned, continuing is not an option.
func (m *stateMutator) ApplyServiceEventsFromValidatedSeals(seals []*flow.Seal) error {
	// We apply service events from blocks which are sealed by this candidate block.
	// The block's payload might contain epoch preparation service events for the next
	// epoch. In this case, we need to update the tentative protocol state.
	// We need to validate whether all information is available in the protocol
	// state to go to the next epoch when needed. In cases where there is a bug
	// in the smart contract, it could be that this happens too late and we should trigger epoch fallback mode.

	// block payload may not specify seals in order, so order them by block height before processing
	orderedSeals, err := protocol.OrderedSeals(seals, m.headers)
	if err != nil {
		// Per API contract, the input seals must have already passed verification, which necessitates
		// successful ordering. Hence, calling protocol.OrderedSeals with the same inputs that succeeded
		// earlier now failed. In all cases, this is an exception.
		return irrecoverable.NewExceptionf("ordering already validated seals unexpectedly failed: %w", err)
	}
	results := make([]*flow.ExecutionResult, 0, len(orderedSeals))
	for _, seal := range orderedSeals {
		result, err := m.results.ByID(seal.ResultID)
		if err != nil {
			return fmt.Errorf("could not get result (id=%x) for seal (id=%x): %w", seal.ResultID, seal.ID(), err)
		}
		results = append(results, result)
	}

	// order all service events in one list
	orderedUpdates := make([]*flow.ServiceEvent, 0)
	for _, result := range results {
		for _, event := range result.ServiceEvents {
			orderedUpdates = append(orderedUpdates, &event)
		}
	}

	for _, stateMachine := range m.orthoKVStoreMachines {
		err := stateMachine.ProcessUpdate(orderedUpdates)
		if err != nil {
			if protocol.IsInvalidServiceEventError(err) {
				// TODO: log, report
				continue
			}
			return fmt.Errorf("could not process service events: %w", err)
		}
	}

	return nil
}
