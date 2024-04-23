package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

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
//
// TODO: Merge methods `EvolveState` and `Build` into one, as they must be always called in this succession (improves API's safety & clarity).
//
//	Temporarily, the stateMutator tracks internally that `EvolveState` is called before `Build` and errors otherwise.
type stateMutator struct {
	headers          storage.Headers
	results          storage.ExecutionResults
	kvStoreSnapshots storage.ProtocolKVStore

	kvMutator            protocol_state.KVStoreMutator
	orthoKVStoreMachines []protocol_state.KeyValueStoreStateMachine
}

var _ StateMutator = (*stateMutator)(nil)

// newStateMutator creates a new instance of stateMutator.
// stateMutator performs initialization of state machine depending on the operation mode of the protocol.
// No errors are expected during normal operations.
func newStateMutator(
	headers storage.Headers,
	results storage.ExecutionResults,
	kvStoreSnapshots storage.ProtocolKVStore,
	candidateView uint64,
	parentID flow.Identifier,
	parentState protocol_state.KVStoreAPI,
	stateMachineFactories ...protocol_state.KeyValueStoreStateMachineFactory,
) (*stateMutator, error) {
	protocolVersion := parentState.GetProtocolStateVersion()
	if versionUpgrade := parentState.GetVersionUpgrade(); versionUpgrade != nil {
		if candidateView >= versionUpgrade.ActivationView {
			protocolVersion = versionUpgrade.Data
		}
	}

	replicatedState, err := parentState.Replicate(protocolVersion)
	if err != nil {
		return nil, fmt.Errorf("could not replicate parent KV store (version=%d) to protocol version %d: %w",
			parentState.GetProtocolStateVersion(), protocolVersion, err)
	}

	stateMachines := make([]protocol_state.KeyValueStoreStateMachine, 0, len(stateMachineFactories))
	for _, factory := range stateMachineFactories {
		stateMachine, err := factory.Create(candidateView, parentID, parentState, replicatedState)
		if err != nil {
			return nil, fmt.Errorf("could not create state machine: %w", err)
		}
		stateMachines = append(stateMachines, stateMachine)
	}

	return &stateMutator{
		headers:              headers,
		results:              results,
		kvStoreSnapshots:     kvStoreSnapshots,
		orthoKVStoreMachines: stateMachines,
		kvMutator:            replicatedState,
	}, nil
}

func (m *stateMutator) Build() (stateID flow.Identifier, dbUpdates *transaction.DeferredBlockPersist, err error) {
	dbUpdates = transaction.NewDeferredBlockPersist()
	for _, stateMachine := range m.orthoKVStoreMachines {
		dbOps, err := stateMachine.Build()
		if err != nil {
			return flow.ZeroID, transaction.NewDeferredBlockPersist(), fmt.Errorf("unexpected exception building state machine's output state: %w", err)
		}
		dbUpdates.AddIndexingOps(dbOps.Pending())
	}
	stateID = m.kvMutator.ID()
	version, data, err := m.kvMutator.VersionedEncode()
	if err != nil {
		return flow.ZeroID, transaction.NewDeferredBlockPersist(), fmt.Errorf("could not encode protocol state: %w", err)
	}

	// Schedule deferred database operations to index the protocol state by the candidate block's ID
	// and persist the new protocol state (if there are any changes)
	dbUpdates.AddIndexingOp(func(blockID flow.Identifier, tx *transaction.Tx) error {
		return m.kvStoreSnapshots.IndexTx(blockID, stateID)(tx)
	})
	dbUpdates.AddDbOp(operation.SkipDuplicatesTx(m.kvStoreSnapshots.StoreTx(stateID, &storage.KeyValueStoreData{
		Version: version,
		Data:    data,
	})))

	return stateID, dbUpdates, nil
}

func (m *stateMutator) EvolveState(seals []*flow.Seal) error {
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
	orderedUpdates := make([]flow.ServiceEvent, 0)
	for _, result := range results {
		for _, event := range result.ServiceEvents {
			orderedUpdates = append(orderedUpdates, event)
		}
	}

	for _, stateMachine := range m.orthoKVStoreMachines {
		// only exceptions should be propagated
		err := stateMachine.EvolveState(orderedUpdates)
		if err != nil {
			return fmt.Errorf("could not process protocol state change for candidate block: %w", err)
		}
	}

	return nil
}
