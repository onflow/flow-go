package protocol_state

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/module/irrecoverable"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Mutator implements protocol.StateMutator interface.
// It has to be used for each block to update the protocol state, even if there are no state-changing
// service events sealed in candidate block. This requirement is due to the fact that protocol state
// is indexed by block ID, and we need to maintain such index.
type Mutator struct {
	protocolStateDB storage.ProtocolState
	headers         storage.Headers
	results         storage.ExecutionResults
	setups          storage.EpochSetups
	commits         storage.EpochCommits
}

var _ protocol.StateMutator = (*Mutator)(nil)

func NewMutator(protocolStateDB storage.ProtocolState) *Mutator {
	return &Mutator{
		protocolStateDB: protocolStateDB,
	}
}

// CreateUpdater creates a new protocol state updater for the given candidate block.
// Has to be called for each block to correctly index the protocol state.
// Expected errors during normal operations:
//   - `storage.ErrNotFound` if no protocol state for parent block is known.
func (m *Mutator) CreateUpdater(candidateView uint64, parentID flow.Identifier) (protocol.StateUpdater, error) {
	parentState, err := m.protocolStateDB.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve protocol state for block (%v): %w", parentID, err)
	}
	return NewUpdater(candidateView, parentState), nil
}

// CommitProtocolState commits the protocol state updater as part of DB transaction.
// Has to be called for each block to correctly index the protocol state.
// No errors are expected during normal operations.
func (m *Mutator) CommitProtocolState(updater protocol.StateUpdater) func(tx *transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		updatedState, updatedStateID, hasChanges := updater.Build()
		if hasChanges {
			err := m.protocolStateDB.StoreTx(updatedStateID, updatedState)(tx)
			if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
				return fmt.Errorf("could not store protocol state (%v): %w", updatedStateID, err)
			}
		}

		err := m.protocolStateDB.Index(updater.Block().ID(), updatedStateID)(tx)
		if err != nil {
			return fmt.Errorf("could not index protocol state (%v) for block (%v): %w",
				updatedStateID, updater.Block().ID(), err)
		}
		return nil
	}
}

// handleEpochServiceEvents handles applying state changes which occur as a result
// of service events being included in a block payload:
//   - inserting incorporated service events
//   - updating EpochStatus for the candidate block
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
// Return values:
//   - dbUpdates - If the service events are valid, or there are no service events,
//     this method returns a slice of Badger operations to apply while storing the block.
//     This includes an operation to index the epoch status for every block, and
//     operations to insert service events for blocks that include them.
//
// No errors are expected during normal operation.
func (m *Mutator) ApplyServiceEvents(updater protocol.StateUpdater, seals []*flow.Seal) (dbUpdates []func(*transaction.Tx) error, err error) {
	epochFallbackTriggered, err := m.isEpochEmergencyFallbackTriggered()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch fallback status: %w", err)
	}

	parentProtocolState := updater.ParentState()
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
		if updater.View() > activeSetup.FinalView {
			// TODO: this is a temporary workaround to allow for the epoch transition to be triggered
			// most likely it will be not needed when we refactor protocol state entries and define strict safety rules.
			err = updater.TransitionToNextEpoch()
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
				// validate the service event
				err := isValidExtendingEpochSetup(ev, activeSetup, epochStatus)
				if err != nil {
					if protocol.IsInvalidServiceEventError(err) {
						// we have observed an invalid service event, which triggers epoch fallback mode
						updater.SetInvalidStateTransitionAttempted()
						return dbUpdates, nil
					}
					return nil, fmt.Errorf("unexpected error validating EpochSetup service event: %w", err)
				}

				err = updater.ProcessEpochSetup(ev)
				if err != nil {
					return nil, irrecoverable.NewExceptionf("could not process epoch setup event: %w", err)
				}

				// we'll insert the setup event when we insert the block
				dbUpdates = append(dbUpdates, m.setups.StoreTx(ev))

			case *flow.EpochCommit:
				// if we receive an EpochCommit event, we must have already observed an EpochSetup event
				// => otherwise, we have observed an EpochCommit without corresponding EpochSetup, which triggers epoch fallback mode
				if epochStatus.NextEpoch.SetupID == flow.ZeroID {
					updater.SetInvalidStateTransitionAttempted()
					return dbUpdates, nil
				}
				extendingSetup := parentProtocolState.NextEpochSetup

				// validate the service event
				err = isValidExtendingEpochCommit(ev, extendingSetup, activeSetup, epochStatus)
				if err != nil {
					if protocol.IsInvalidServiceEventError(err) {
						// we have observed an invalid service event, which triggers epoch fallback mode
						updater.SetInvalidStateTransitionAttempted()
						return dbUpdates, nil
					}
					return nil, fmt.Errorf("unexpected error validating EpochCommit service event: %w", err)
				}

				err = updater.ProcessEpochCommit(ev)
				if err != nil {
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
