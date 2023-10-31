package protocol_state

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// stateMutator implements protocol.StateMutator interface.
// It has to be used for each block to update the protocol state, even if there are no state-changing
// service events sealed in candidate block. This requirement is due to the fact that protocol state
// is indexed by block ID, and we need to maintain such index.
type stateMutator struct {
	headers          storage.Headers
	results          storage.ExecutionResults
	setups           storage.EpochSetups
	commits          storage.EpochCommits
	stateMachine     ProtocolStateMachine
	params           protocol.InstanceParams
	pendingDbUpdates []func(tx *transaction.Tx) error
}

var _ protocol.StateMutator = (*stateMutator)(nil)

func newStateMutator(
	headers storage.Headers,
	results storage.ExecutionResults,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	stateMachine ProtocolStateMachine,
	params protocol.InstanceParams,
) *stateMutator {
	return &stateMutator{
		setups:       setups,
		params:       params,
		headers:      headers,
		results:      results,
		commits:      commits,
		stateMachine: stateMachine,
	}
}

// Build returns:
//   - hasChanges: flag whether there were any changes; otherwise, `updatedState` and `stateID` equal the parent state
//   - updatedState: the ProtocolState after applying all updates
//   - stateID: the has commitment to the `updatedState`
//   - dbUpdates: database updates necessary for persisting the updated protocol state
//
// updated protocol state entry, state ID and a flag indicating if there were any changes.
func (m *stateMutator) Build() (hasChanges bool, updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, dbUpdates []func(tx *transaction.Tx) error) {
	updatedState, stateID, hasChanges = m.stateMachine.Build()
	dbUpdates = m.pendingDbUpdates
	return
}

// ApplyServiceEvents handles applying state changes which occur as a result
// of service events being included in a block payload.
// All updates that are incorporated in service events are applied to the protocol state by mutating protocol state updater.
// No errors are expected during normal operation.
func (m *stateMutator) ApplyServiceEvents(seals []*flow.Seal) error {
	dbUpdates, err := m.handleServiceEvents(seals)
	if err != nil {
		return err
	}
	m.pendingDbUpdates = append(m.pendingDbUpdates, dbUpdates...)
	return nil
}

// handleServiceEvents handles applying state changes which occur as a result
// of service events being included in a block payload:
//   - inserting incorporated service events
//   - updating protocol state for the candidate block
//
// Consider a chain where a service event is emitted during execution of block A.
// Block B contains a receipt for A. Block C contains a seal for block A.
//
// A <- .. <- B(RA) <- .. <- C(SA)
//
// Service events are included within execution results, which are stored
// opaquely as part of the block payload in block B. We only validate and insert
// the typed service event to storage once we process C, the block containing the
// seal for block A. This is because we rely on the sealing subsystem to validate
// correctness of the service event before processing it.
// Consequently, any change to the protocol state introduced by a service event
// emitted during execution of block A would only become visible when querying
// C or its descendants.
//
// This method will only apply service-event-induced state changes when the
// input block has the form of block C (ie. contains a seal for a block in
// which a service event was emitted).
//
// All updates that are incorporated in service events are applied to the protocol state by mutating protocol state updater.
// This method doesn't modify any data from self, all protocol state changes are applied to state updater.
// All other changes are returned as slice of deferred DB updates.
//
// Return values:
//   - dbUpdates - If the service events are valid, or there are no service events,
//     this method returns a slice of Badger operations to apply while storing the block.
//     This includes operations to insert service events for blocks that include them.
//
// No errors are expected during normal operation.
func (m *stateMutator) handleServiceEvents(seals []*flow.Seal) (dbUpdates []func(*transaction.Tx) error, err error) {
	epochFallbackTriggered, err := m.params.EpochFallbackTriggered()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch fallback status: %w", err)
	}

	parentProtocolState := m.stateMachine.ParentState()
	epochStatus := parentProtocolState.EpochStatus()
	activeSetup := parentProtocolState.CurrentEpochSetup

	// never process service events after epoch fallback is triggered
	if epochStatus.InvalidServiceEventIncorporated || epochFallbackTriggered {
		return dbUpdates, nil
	}

	// perform protocol state transition to next epoch if next epoch is committed and we are at first block of epoch
	phase, err := epochStatus.Phase()
	if err != nil {
		return nil, fmt.Errorf("could not determine epoch phase: %w", err)
	}
	if phase == flow.EpochPhaseCommitted {
		if m.stateMachine.View() > activeSetup.FinalView {
			// TODO: this is a temporary workaround to allow for the epoch transition to be triggered
			// most likely it will be not needed when we refactor protocol state entries and define strict safety rules.
			err = m.stateMachine.TransitionToNextEpoch()
			if err != nil {
				return nil, fmt.Errorf("could not transition protocol state to next epoch: %w", err)
			}
		}
	}

	// We apply service events from blocks which are sealed by this candidate block.
	// The block's payload might contain epoch preparation service events for the next
	// epoch. In this case, we need to update the tentative protocol state.
	// We need to validate whether all information is available in the protocol
	// state to go to the next epoch when needed. In cases where there is a bug
	// in the smart contract, it could be that this happens too late and the
	// chain finalization should halt.

	// block payload may not specify seals in order, so order them by block height before processing
	orderedSeals, err := protocol.OrderedSeals(seals, m.headers)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("ordering seals: parent payload contains seals for unknown block: %s", err.Error())
		}
		return nil, fmt.Errorf("unexpected error ordering seals: %w", err)
	}
	for _, seal := range orderedSeals {
		result, err := m.results.ByID(seal.ResultID)
		if err != nil {
			return nil, fmt.Errorf("could not get result (id=%x) for seal (id=%x): %w", seal.ResultID, seal.ID(), err)
		}

		for _, event := range result.ServiceEvents {

			switch ev := event.Event.(type) {
			case *flow.EpochSetup:
				err = m.stateMachine.ProcessEpochSetup(ev)
				if err != nil {
					if protocol.IsInvalidServiceEventError(err) {
						// we have observed an invalid service event, which triggers epoch fallback mode
						m.stateMachine.SetInvalidStateTransitionAttempted()
						return dbUpdates, nil
					}
					return nil, irrecoverable.NewExceptionf("could not process epoch setup event: %w", err)
				}

				// we'll insert the setup event when we insert the block
				dbUpdates = append(dbUpdates, m.setups.StoreTx(ev))

			case *flow.EpochCommit:
				err = m.stateMachine.ProcessEpochCommit(ev)
				if err != nil {
					if protocol.IsInvalidServiceEventError(err) {
						// we have observed an invalid service event, which triggers epoch fallback mode
						m.stateMachine.SetInvalidStateTransitionAttempted()
						return dbUpdates, nil
					}
					return nil, irrecoverable.NewExceptionf("could not process epoch commit event: %w", err)
				}

				// we'll insert the commit event when we insert the block
				dbUpdates = append(dbUpdates, m.commits.StoreTx(ev))
			case *flow.VersionBeacon:
				// do nothing for now
			default:
				return nil, fmt.Errorf("invalid service event type (type_name=%s, go_type=%T)", event.Type, ev)
			}
		}
	}
	return
}
