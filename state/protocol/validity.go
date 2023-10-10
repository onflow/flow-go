package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
)

// IsValidExtendingEpochSetup checks whether an epoch setup service being
// added to the state is valid. In addition to intrinsic validity, we also
// check that it is valid w.r.t. the previous epoch setup event, and the
// current epoch status.
// Assumes all inputs besides extendingSetup are already validated.
// Expected errors during normal operations:
// * protocol.InvalidServiceEventError if the input service event is invalid to extend the currently active epoch status
func IsValidExtendingEpochSetup(extendingSetup *flow.EpochSetup, activeSetup *flow.EpochSetup, status *flow.EpochStatus) error {
	// We should only have a single epoch setup event per epoch.
	if status.NextEpoch.SetupID != flow.ZeroID {
		// true iff EpochSetup event for NEXT epoch was already included before
		return NewInvalidServiceEventErrorf("duplicate epoch setup service event: %x", status.NextEpoch.SetupID)
	}

	// The setup event should have the counter increased by one.
	if extendingSetup.Counter != activeSetup.Counter+1 {
		return NewInvalidServiceEventErrorf("next epoch setup has invalid counter (%d => %d)", activeSetup.Counter, extendingSetup.Counter)
	}

	// The first view needs to be exactly one greater than the current epoch final view
	if extendingSetup.FirstView != activeSetup.FinalView+1 {
		return NewInvalidServiceEventErrorf(
			"next epoch first view must be exactly 1 more than current epoch final view (%d != %d+1)",
			extendingSetup.FirstView,
			activeSetup.FinalView,
		)
	}

	// Finally, the epoch setup event must contain all necessary information.
	err := VerifyEpochSetup(extendingSetup, true)
	if err != nil {
		return NewInvalidServiceEventErrorf("invalid epoch setup: %w", err)
	}

	return nil
}

// VerifyEpochSetup checks whether an `EpochSetup` event is syntactically correct.
// The boolean parameter `verifyNetworkAddress` controls, whether we want to permit
// nodes to share a networking address.
// This is a side-effect-free function. Any error return indicates that the
// EpochSetup event is not compliant with protocol rules.
func VerifyEpochSetup(setup *flow.EpochSetup, verifyNetworkAddress bool) error {
	// STEP 1: general sanity checks
	// the seed needs to be at least minimum length
	if len(setup.RandomSource) != flow.EpochSetupRandomSourceLength {
		return fmt.Errorf("seed has incorrect length (%d != %d)", len(setup.RandomSource), flow.EpochSetupRandomSourceLength)
	}

	// STEP 2: sanity checks of all nodes listed as participants
	// there should be no duplicate node IDs
	identLookup := make(map[flow.Identifier]struct{})
	for _, participant := range setup.Participants {
		_, ok := identLookup[participant.NodeID]
		if ok {
			return fmt.Errorf("duplicate node identifier (%x)", participant.NodeID)
		}
		identLookup[participant.NodeID] = struct{}{}
	}

	if verifyNetworkAddress {
		// there should be no duplicate node addresses
		addrLookup := make(map[string]struct{})
		for _, participant := range setup.Participants {
			_, ok := addrLookup[participant.Address]
			if ok {
				return fmt.Errorf("duplicate node address (%x)", participant.Address)
			}
			addrLookup[participant.Address] = struct{}{}
		}
	}

	// the participants must be listed in canonical order
	if !setup.Participants.Sorted(order.Canonical) {
		return fmt.Errorf("participants are not canonically ordered")
	}

	// STEP 3: sanity checks for individual roles
	// IMPORTANT: here we remove all nodes with zero weight, as they are allowed to partake
	// in communication but not in respective node functions
	activeParticipants := setup.Participants.Filter(filter.HasWeight(true))

	// we need at least one node of each role
	roles := make(map[flow.Role]uint)
	for _, participant := range activeParticipants {
		roles[participant.Role]++
	}
	if roles[flow.RoleConsensus] < 1 {
		return fmt.Errorf("need at least one consensus node")
	}
	if roles[flow.RoleCollection] < 1 {
		return fmt.Errorf("need at least one collection node")
	}
	if roles[flow.RoleExecution] < 1 {
		return fmt.Errorf("need at least one execution node")
	}
	if roles[flow.RoleVerification] < 1 {
		return fmt.Errorf("need at least one verification node")
	}

	// first view must be before final view
	if setup.FirstView >= setup.FinalView {
		return fmt.Errorf("first view (%d) must be before final view (%d)", setup.FirstView, setup.FinalView)
	}

	// we need at least one collection cluster
	if len(setup.Assignments) == 0 {
		return fmt.Errorf("need at least one collection cluster")
	}

	// the collection cluster assignments need to be valid
	_, err := factory.NewClusterList(setup.Assignments, activeParticipants.Filter(filter.HasRole(flow.RoleCollection)))
	if err != nil {
		return fmt.Errorf("invalid cluster assignments: %w", err)
	}

	return nil
}

// IsValidExtendingEpochCommit checks whether an epoch commit service being
// added to the state is valid. In addition to intrinsic validity, we also
// check that it is valid w.r.t. the previous epoch setup event, and the
// current epoch status.
// Assumes all inputs besides extendingCommit are already validated.
// Expected errors during normal operations:
// * protocol.InvalidServiceEventError if the input service event is invalid to extend the currently active epoch status
func IsValidExtendingEpochCommit(extendingCommit *flow.EpochCommit, extendingSetup *flow.EpochSetup, activeSetup *flow.EpochSetup, status *flow.EpochStatus) error {

	// We should only have a single epoch commit event per epoch.
	if status.NextEpoch.CommitID != flow.ZeroID {
		// true iff EpochCommit event for NEXT epoch was already included before
		return NewInvalidServiceEventErrorf("duplicate epoch commit service event: %x", status.NextEpoch.CommitID)
	}

	// The epoch setup event needs to happen before the commit.
	if status.NextEpoch.SetupID == flow.ZeroID {
		return NewInvalidServiceEventErrorf("missing epoch setup for epoch commit")
	}

	// The commit event should have the counter increased by one.
	if extendingCommit.Counter != activeSetup.Counter+1 {
		return NewInvalidServiceEventErrorf("next epoch commit has invalid counter (%d => %d)", activeSetup.Counter, extendingCommit.Counter)
	}

	err := IsValidEpochCommit(extendingCommit, extendingSetup)
	if err != nil {
		return NewInvalidServiceEventErrorf("invalid epoch commit: %s", err)
	}

	return nil
}

// IsValidEpochCommit checks whether an epoch commit service event is intrinsically valid.
// Assumes the input flow.EpochSetup event has already been validated.
// Expected errors during normal operations:
// * protocol.InvalidServiceEventError if the EpochCommit is invalid
func IsValidEpochCommit(commit *flow.EpochCommit, setup *flow.EpochSetup) error {

	if len(setup.Assignments) != len(commit.ClusterQCs) {
		return NewInvalidServiceEventErrorf("number of clusters (%d) does not number of QCs (%d)", len(setup.Assignments), len(commit.ClusterQCs))
	}

	if commit.Counter != setup.Counter {
		return NewInvalidServiceEventErrorf("inconsistent epoch counter between commit (%d) and setup (%d) events in same epoch", commit.Counter, setup.Counter)
	}

	// make sure we have a valid DKG public key
	if commit.DKGGroupKey == nil {
		return NewInvalidServiceEventErrorf("missing DKG public group key")
	}

	participants := setup.Participants.Filter(filter.IsValidDKGParticipant)
	if len(participants) != len(commit.DKGParticipantKeys) {
		return NewInvalidServiceEventErrorf("participant list (len=%d) does not match dkg key list (len=%d)", len(participants), len(commit.DKGParticipantKeys))
	}

	return nil
}
