package kvstore

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

// PSVersionUpgradeStateMachine encapsulates the logic for evolving the version of the Protocol State.
// Specifically, it consumes ProtocolStateVersionUpgrade ServiceEvent that are sealed by the candidate block
// (possibly still under construction) with the given view.
// Each relevant event is validated before it is applied to the KV store.
// All updates are applied to a copy of parent KV store, so parent KV store is not modified.
// A separate instance should be created for each block to process the updates therein.
type PSVersionUpgradeStateMachine struct {
	candidateView uint64
	parentState   protocol_state.KVStoreReader
	mutator       protocol_state.KVStoreMutator
	params        protocol.GlobalParams
}

var _ protocol_state.KeyValueStoreStateMachine = (*PSVersionUpgradeStateMachine)(nil)

// NewPSVersionUpgradeStateMachine creates a new state machine to update a specific sub-state of the KV Store.
// It schedules protocol state version upgrades upon receiving a `ProtocolStateVersionUpgrade` event.
// The actual model upgrade is handled in the upper layer (`ProtocolStateMachine`).
func NewPSVersionUpgradeStateMachine(
	candidateView uint64,
	params protocol.GlobalParams,
	parentState protocol_state.KVStoreReader,
	mutator protocol_state.KVStoreMutator,
) *PSVersionUpgradeStateMachine {
	return &PSVersionUpgradeStateMachine{
		candidateView: candidateView,
		parentState:   parentState,
		mutator:       mutator,
		params:        params,
	}
}

// Build is a no-op, because scheduled version upgrades are stored in the KVStore only.
// (There is no secondary database operations to persist version upgrade data.)
func (m *PSVersionUpgradeStateMachine) Build() (*protocol.DeferredBlockPersist, error) {
	return protocol.NewDeferredBlockPersist(), nil
}

// EvolveState applies the state change(s) on sub-state P for the candidate block (under construction).
// Implementation processes only relevant service events and ignores all other events.
// No errors are expected during normal operations.
func (m *PSVersionUpgradeStateMachine) EvolveState(orderedUpdates []flow.ServiceEvent) error {
	for _, update := range orderedUpdates {
		switch update.Type {
		case flow.ServiceEventProtocolStateVersionUpgrade:
			versionUpgrade, ok := update.Event.(*flow.ProtocolStateVersionUpgrade)
			if !ok {
				return fmt.Errorf("internal invalid type for ProtocolStateVersionUpgrade: %T", update.Event)
			}

			err := m.processSingleEvent(versionUpgrade)
			if err != nil {
				if protocol.IsInvalidServiceEventError(err) {
					// TODO: log, report invalid service event
					continue
				}
				return fmt.Errorf("unexpected error when processing version upgrade event: %w", err)
			}

		// Service events not explicitly expected are ignored
		default:
			continue
		}
	}

	return nil
}

// processSingleEvent performs processing of a single protocol version upgrade event.
// Expected errors indicating that we have observed and invalid service event from protocol's point of view.
//   - `protocol.InvalidServiceEventError` - if the service event is invalid for the current protocol state.
//
// All other errors should be treated as exceptions.
func (m *PSVersionUpgradeStateMachine) processSingleEvent(versionUpgrade *flow.ProtocolStateVersionUpgrade) error {
	// To switch the protocol version, replica needs to process a block with a candidateView >= activation view.
	// But we cannot activate a new version till the block containing the seal is finalized, because when
	// switching between forks with different highest views, we do not want to switch forth and back between versions.
	// The problem is that finality is local to each node due to the nature of the consensus algorithm itself.
	// We would like to guarantee that all nodes switch the protocol version at exactly the same block.
	// To guarantee that all nodes switch the protocol version at exactly the same block, we require that the
	// activation view is higher than the candidateView + Δ when accepting the event. Δ represents the finalization lag
	// to give time for replicas to finalize the block containing the seal for the version upgrade event.
	// When replica reaches (or exceeds) the activation view *and* the latest finalized protocol state knows
	// about the version upgrade, only then it's safe to switch the protocol version.
	if m.candidateView+m.params.EpochCommitSafetyThreshold() >= versionUpgrade.ActiveView {
		return protocol.NewInvalidServiceEventErrorf("view %d triggering version upgrade must be at least %d views in the future of current view %d: %w",
			versionUpgrade.ActiveView, m.params.EpochCommitSafetyThreshold(), m.candidateView, ErrInvalidActivationView)
	}

	if m.parentState.GetProtocolStateVersion()+1 != versionUpgrade.NewProtocolStateVersion {
		return protocol.NewInvalidServiceEventErrorf("invalid protocol state version upgrade %d -> %d: %w",
			m.parentState.GetProtocolStateVersion(), versionUpgrade.NewProtocolStateVersion, ErrInvalidUpgradeVersion)
	}

	// checkPendingUpgrade checks if there is a pending upgrade in the state and validates if we can accept the upgrade request.
	// We allow setting version upgrade if all of the following conditions are met:
	// (i) the activation view is bigger than or equal to the current candidate block's view + Δ.
	// (ii) if there is a pending upgrade, the new version should be the same as the pending upgrade.
	// Condition (ii) is checked in this function.
	checkPendingUpgrade := func(store protocol_state.KVStoreReader) error {
		if pendingUpgrade := store.GetVersionUpgrade(); pendingUpgrade != nil {
			if pendingUpgrade.ActivationView < m.candidateView {
				// pending upgrade has been activated, we can ignore it.
				return nil
			}

			// we allow updating pending upgrade iff the new version is the same as the pending upgrade
			// the activation view may differ, but it has to meet the same threshold.
			if pendingUpgrade.Data != versionUpgrade.NewProtocolStateVersion {
				return protocol.NewInvalidServiceEventErrorf("requested to upgrade to %d but pending upgrade with version already stored %d: %w",
					pendingUpgrade.Data, versionUpgrade.NewProtocolStateVersion, ErrInvalidUpgradeVersion)
			}
		}
		return nil
	}

	// There can be multiple `versionUpgrade` Service Events sealed in one block. In case we have _not_
	// encountered any, `m.mutator` contains the latest `versionUpgrade` as of the parent block, because
	// we cloned it from the parent state. If we encountered some version upgrades, we already enforced that
	// they are all upgrades to the same version. So we only need to check that the next `versionUpgrade`
	// also has the same version.
	err := checkPendingUpgrade(m.mutator)
	if err != nil {
		return fmt.Errorf("version upgrade invalid with respect to the current state: %w", err)
	}

	activator := &protocol_state.ViewBasedActivator[uint64]{
		Data:           versionUpgrade.NewProtocolStateVersion,
		ActivationView: versionUpgrade.ActiveView,
	}
	m.mutator.SetVersionUpgrade(activator)
	return nil
}

// View returns the view associated with this state machine.
// The view of the state machine equals the view of the block carrying the respective updates.
func (m *PSVersionUpgradeStateMachine) View() uint64 {
	return m.candidateView
}

// ParentState returns parent state associated with this state machine.
func (m *PSVersionUpgradeStateMachine) ParentState() protocol_state.KVStoreReader {
	return m.parentState
}
