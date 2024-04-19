package state

import (
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
// It is backed by a storage.ProtocolState and an in-memory protocol.GlobalParams.
type ProtocolState struct {
	epochProtocolStateDB storage.ProtocolState
	kvStoreSnapshots     storage.ProtocolKVStore
	globalParams         protocol.GlobalParams
}

var _ protocol.ProtocolState = (*ProtocolState)(nil)

func NewProtocolState(protocolStateDB storage.ProtocolState, kvStoreSnapshots storage.ProtocolKVStore, globalParams protocol.GlobalParams) *ProtocolState {
	return &ProtocolState{
		epochProtocolStateDB: protocolStateDB,
		kvStoreSnapshots:     kvStoreSnapshots,
		globalParams:         globalParams,
	}
}

// AtBlockID returns epoch protocol state at block ID.
// The resulting epoch protocol state is returned AFTER applying updates that are contained in block.
// Can be queried for any block that has been added to the block tree.
// Returns:
// - (DynamicProtocolState, nil) - if there is an epoch protocol state associated with given block ID.
// - (nil, storage.ErrNotFound) - if there is no epoch protocol state associated with given block ID.
// - (nil, exception) - any other error should be treated as exception.
func (s *ProtocolState) AtBlockID(blockID flow.Identifier) (protocol.DynamicProtocolState, error) {
	protocolStateEntry, err := s.epochProtocolStateDB.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not query epoch protocol state at block (%x): %w", blockID, err)
	}
	return inmem.NewDynamicProtocolStateAdapter(protocolStateEntry, s.globalParams), nil
}

// KVStoreAtBlockID returns protocol state at block ID.
// The resulting protocol state is returned AFTER applying updates that are contained in block.
// Can be queried for any block that has been added to the block tree.
// Returns:
// - (KVStoreReader, nil) - if there is a protocol state associated with given block ID.
// - (nil, storage.ErrNotFound) - if there is no protocol state associated with given block ID.
// - (nil, exception) - any other error should be treated as exception.
func (s *ProtocolState) KVStoreAtBlockID(blockID flow.Identifier) (protocol.KVStoreReader, error) {
	return s.kvStoreAtBlockID(blockID)
}

// kvStoreAtBlockID queries KV store by block ID and decodes it from binary data to a typed interface.
// Returns:
// - (protocol_state.KVStoreAPI, nil) - if there is a protocol state associated with given block ID.
// - (nil, storage.ErrNotFound) - if there is no protocol state associated with given block ID.
// - (nil, exception) - any other error should be treated as exception.
func (s *ProtocolState) kvStoreAtBlockID(blockID flow.Identifier) (protocol_state.KVStoreAPI, error) {
	versionedData, err := s.kvStoreSnapshots.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not query KV store at block (%x): %w", blockID, err)
	}
	kvStore, err := kvstore.VersionedDecode(versionedData.Version, versionedData.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decode protocol state (version=%d) at block (%x): %w",
			versionedData.Version, blockID, err)
	}
	return kvStore, err
}

// GlobalParams returns an interface which can be used to query global protocol parameters.
func (s *ProtocolState) GlobalParams() protocol.GlobalParams {
	return s.globalParams
}

// MutableProtocolState is an implementation of the mutable interface for protocol state, it allows to evolve the protocol state
// by acting as factory for protocol.StateMutator which can be used to apply state-changing operations.
type MutableProtocolState struct {
	ProtocolState
	headers          storage.Headers
	results          storage.ExecutionResults
	kvStoreFactories []protocol_state.KeyValueStoreStateMachineFactory
}

var _ protocol.MutableProtocolState = (*MutableProtocolState)(nil)

// NewMutableProtocolState creates a new instance of MutableProtocolState.
func NewMutableProtocolState(
	epochProtocolStateDB storage.ProtocolState,
	kvStoreSnapshots storage.ProtocolKVStore,
	globalParams protocol.GlobalParams,
	headers storage.Headers,
	results storage.ExecutionResults,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
) *MutableProtocolState {
	// an ordered list of factories to create state machines for different sub-states of the Dynamic Protocol State.
	// all factories are expected to be called in order defined here.
	kvStoreFactories := []protocol_state.KeyValueStoreStateMachineFactory{
		kvstore.NewPSVersionUpgradeStateMachineFactory(globalParams),
		epochs.NewEpochStateMachineFactory(globalParams, setups, commits, epochProtocolStateDB),
	}
	return newMutableProtocolState(epochProtocolStateDB, kvStoreSnapshots, globalParams, headers, results, kvStoreFactories)
}

// newMutableProtocolState creates a new instance of MutableProtocolState, where we inject factories for the orthogonal
// state machines evolving the sub-states. This constructor should be used mainly for testing (hence it is not exported).
// Specifically, the MutableProtocolState is conceptually independent of the specific functions that the state machines
// implement. Therefore, we test it independently of the state machines required for production. In comparison, the
// constructor `NewMutableProtocolState` is intended for production use, where the list of state machines is hard-coded.
func newMutableProtocolState(
	epochProtocolStateDB storage.ProtocolState,
	kvStoreSnapshots storage.ProtocolKVStore,
	globalParams protocol.GlobalParams,
	headers storage.Headers,
	results storage.ExecutionResults,
	kvStoreFactories []protocol_state.KeyValueStoreStateMachineFactory,
) *MutableProtocolState {
	return &MutableProtocolState{
		ProtocolState:    *NewProtocolState(epochProtocolStateDB, kvStoreSnapshots, globalParams),
		headers:          headers,
		results:          results,
		kvStoreFactories: kvStoreFactories,
	}
}

// EvolveState
// Has to be called for each block to evolve the protocol state.
// Expected errors during normal operations:
func (s *MutableProtocolState) EvolveState(
	parentStateID flow.Identifier,
	candidateView uint64,
	candidateSeals []*flow.Seal,
) (flow.Identifier, *transaction.DeferredBlockPersist, error) {
	serviceEvents, err := s.serviceEventsFromSeals(candidateSeals)
	if err != nil {
		return flow.ZeroID, transaction.NewDeferredBlockPersist(), irrecoverable.NewExceptionf("extracting service events from candidate seals failed: %w", err)
	}

	stateMachines, evolvingState, err := s.initializeOrthogonalStateMachines(parentStateID, candidateView)
	if err != nil {
		return flow.ZeroID, transaction.NewDeferredBlockPersist(), irrecoverable.NewExceptionf("failure initializing sub-state machines for evolving the Protocol State: %w", err)
	}

	resultingStateID, dbUpdates, err := s.build(parentStateID, stateMachines, serviceEvents, evolvingState)
	if err != nil {
		return flow.ZeroID, transaction.NewDeferredBlockPersist(), irrecoverable.NewExceptionf("evolving and building the resulting Protocol State failed: %w", err)
	}
	return resultingStateID, dbUpdates, nil
}

// initializeOrthogonalStateMachines
// TODO: Documentation
func (s *MutableProtocolState) initializeOrthogonalStateMachines(
	parentStateID flow.Identifier,
	candidateView uint64,
) ([]protocol_state.KeyValueStoreStateMachine, protocol_state.KVStoreMutator, error) {

	parentState, err := s.kvStoreAtBlockID(parentStateID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve parent Protocol State %v: %w", parentStateID, err)
	}

	protocolVersion := parentState.GetProtocolStateVersion()
	if versionUpgrade := parentState.GetVersionUpgrade(); versionUpgrade != nil {
		if candidateView >= versionUpgrade.ActivationView {
			protocolVersion = versionUpgrade.Data
		}
	}

	evolvingState, err := parentState.Replicate(protocolVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("could not replicate parent KV store (version=%d) to protocol version %d: %w", parentState.GetProtocolStateVersion(), protocolVersion, err)
	}

	stateMachines := make([]protocol_state.KeyValueStoreStateMachine, 0, len(s.kvStoreFactories))
	for _, factory := range s.kvStoreFactories {
		stateMachine, err := factory.Create(candidateView, parentStateID, parentState, evolvingState)
		if err != nil {
			return nil, nil, fmt.Errorf("could not create state machine: %w", err)
		}
		stateMachines = append(stateMachines, stateMachine)
	}
	return stateMachines, evolvingState, nil
}

// serviceEventsFromSeals
// TODO: Documentation
func (s *MutableProtocolState) serviceEventsFromSeals(candidateSeals []*flow.Seal) ([]flow.ServiceEvent, error) {
	// block payload may not specify seals in order, so order them by block height before processing
	orderedSeals, err := protocol.OrderedSeals(candidateSeals, s.headers)
	if err != nil {
		// Per API contract, the input seals must have already passed verification, which necessitates
		// successful ordering. Hence, calling protocol.OrderedSeals with the same inputs that succeeded
		// earlier now failed. In all cases, this is an exception.
		return nil, fmt.Errorf("ordering already validated seals unexpectedly failed: %w", err)
	}

	serviceEvents := make([]flow.ServiceEvent, 0) // we expect that service events are rare; most blocks have none
	for _, seal := range orderedSeals {
		result, err := s.results.ByID(seal.ResultID)
		if err != nil {
			return nil, fmt.Errorf("could not get result (id=%x) for seal (id=%x): %w", seal.ResultID, seal.ID(), err)
		}
		serviceEvents = append(serviceEvents, result.ServiceEvents...)
	}
	return serviceEvents, nil
}

// build assembled the final Protocol State.
// First, we apply the service events to all sub-state machines and then build the resulting state.
// Thereby, the framework supports a subtly more general way of partitioning the Protocol State machine,
// where state machines could exchange some information, if their chronological oder pof execution is strictly
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
			return flow.ZeroID, transaction.NewDeferredBlockPersist(), fmt.Errorf("exception from sub-state machine during state evolution: %w", err)
		}
	}

	// _after_ all state machines have ingested the available information, we build the resulting overall state
	dbUpdates := transaction.NewDeferredBlockPersist()
	for _, stateMachine := range stateMachines {
		dbOps, err := stateMachine.Build()
		if err != nil {
			return flow.ZeroID, transaction.NewDeferredBlockPersist(), fmt.Errorf("unexpected exception building state machine's output state: %w", err)
		}
		dbUpdates.AddIndexingOps(dbOps.Pending())
	}
	resultingStateID := evolvingState.ID()
	version, data, err := evolvingState.VersionedEncode()
	if err != nil {
		return flow.ZeroID, transaction.NewDeferredBlockPersist(), fmt.Errorf("could not encode resutling protocol state: %w", err)
	}

	// deferred dbUpdate for indexing the protocol state by the candidate block's ID:
	dbUpdates.AddIndexingOp(func(blockID flow.Identifier, tx *transaction.Tx) error {
		return s.kvStoreSnapshots.IndexTx(blockID, resultingStateID)(tx)
	})
	// deferred dbUpdate: persist new protocol state iff it has changed (otherwise, we have already persisted it for the parent block)
	if parentStateID != resultingStateID {
		dbUpdates.AddDbOp(operation.SkipDuplicatesTx(s.kvStoreSnapshots.StoreTx(resultingStateID, &storage.KeyValueStoreData{
			Version: version,
			Data:    data,
		})))
	}

	return resultingStateID, dbUpdates, nil
}
