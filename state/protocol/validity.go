package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
)

// IsValidExtendingEpochSetup checks whether an EpochSetup service event being added to the state is valid.
// In addition to intrinsic validity, we also check that it is valid w.r.t. the previous epoch setup event,
// and the current epoch status.
// CAUTION: This function assumes that all inputs besides extendingCommit are already validated.
// Expected errors during normal operations:
// * protocol.InvalidServiceEventError if the input service event is invalid to extend the currently active epoch status
func IsValidExtendingEpochSetup(extendingSetup *flow.EpochSetup, protocolStateEntry *flow.ProtocolStateEntry, currentEpochSetupEvent *flow.EpochSetup) error {
	// Enforce EpochSetup is valid w.r.t to current epoch state
	if protocolStateEntry.NextEpoch != nil { // We should only have a single epoch setup event per epoch.
		// true iff EpochSetup event for NEXT epoch was already included before
		return NewInvalidServiceEventErrorf("duplicate epoch setup service event: %x", protocolStateEntry.NextEpoch.SetupID)
	}
	if extendingSetup.Counter != currentEpochSetupEvent.Counter+1 { // The setup event should have the counter increased by one.
		return NewInvalidServiceEventErrorf("next epoch setup has invalid counter (%d => %d)", currentEpochSetupEvent.Counter, extendingSetup.Counter)
	}
	if extendingSetup.FirstView != currentEpochSetupEvent.FinalView+1 { // The first view needs to be exactly one greater than the current epoch final view
		return NewInvalidServiceEventErrorf(
			"next epoch first view must be exactly 1 more than current epoch final view (%d != %d+1)",
			extendingSetup.FirstView,
			currentEpochSetupEvent.FinalView,
		)
	}

	// Enforce the EpochSetup event is syntactically correct
	err := IsValidEpochSetup(extendingSetup, true)
	if err != nil {
		return NewInvalidServiceEventErrorf("invalid epoch setup: %w", err)
	}
	return nil
}

// IsValidEpochSetup checks whether an `EpochSetup` event is syntactically correct. The boolean parameter `verifyNetworkAddress`
// controls, whether we want to permit nodes to share a networking address.
// This is a side-effect-free function. Any error return indicates that the EpochSetup event is not compliant with protocol rules.
func IsValidEpochSetup(setup *flow.EpochSetup, verifyNetworkAddress bool) error {
	// 1. CHECK: Enforce protocol compliance of Epoch parameters:
	// - RandomSource of entropy in Epoch Setup event should the protocol-prescribed length
	// - first view must be before final view
	if len(setup.RandomSource) != flow.EpochSetupRandomSourceLength {
		return fmt.Errorf("seed has incorrect length (%d != %d)", len(setup.RandomSource), flow.EpochSetupRandomSourceLength)
	}
	if setup.FirstView >= setup.FinalView {
		return fmt.Errorf("first view (%d) must be before final view (%d)", setup.FirstView, setup.FinalView)
	}

	// 2. CHECK: Enforce protocol compliance active participants:
	// (a) each has a unique node ID,
	// (b) each has a unique network address (if `verifyNetworkAddress` is true),
	// (c) participants are sorted in canonical order.
	//     Note that the system smart contracts manage the identity table as an unordered set! For the protocol state, we desire a fixed
	//     ordering to simplify various implementation details, like the DKG. Therefore, we order identities in `flow.EpochSetup` during
	//     conversion from cadence to Go in the function `convert.ServiceEvent(flow.ChainID, flow.Event)` in package `model/convert`
	identLookup := make(map[flow.Identifier]struct{})
	for _, participant := range setup.Participants { // (a) enforce uniqueness of NodeIDs
		_, ok := identLookup[participant.NodeID]
		if ok {
			return fmt.Errorf("duplicate node identifier (%x)", participant.NodeID)
		}
		identLookup[participant.NodeID] = struct{}{}
	}

	if verifyNetworkAddress { // (b) enforce uniqueness of networking address
		addrLookup := make(map[string]struct{})
		for _, participant := range setup.Participants {
			_, ok := addrLookup[participant.Address]
			if ok {
				return fmt.Errorf("duplicate node address (%x)", participant.Address)
			}
			addrLookup[participant.Address] = struct{}{}
		}
	}

	if !setup.Participants.Sorted(flow.Canonical[flow.IdentitySkeleton]) { // (c) enforce canonical ordering
		return fmt.Errorf("participants are not canonically ordered")
	}

	// 3. CHECK: Enforce sufficient number of nodes for each role
	// IMPORTANT: here we remove all nodes with zero weight, as they are allowed to partake in communication but not in respective node functions
	activeParticipants := setup.Participants.Filter(filter.HasInitialWeight[flow.IdentitySkeleton](true))
	activeNodeCountByRole := make(map[flow.Role]uint)
	for _, participant := range activeParticipants {
		activeNodeCountByRole[participant.Role]++
	}
	if activeNodeCountByRole[flow.RoleConsensus] < 1 {
		return fmt.Errorf("need at least one consensus node")
	}
	if activeNodeCountByRole[flow.RoleCollection] < 1 {
		return fmt.Errorf("need at least one collection node")
	}
	if activeNodeCountByRole[flow.RoleExecution] < 1 {
		return fmt.Errorf("need at least one execution node")
	}
	if activeNodeCountByRole[flow.RoleVerification] < 1 {
		return fmt.Errorf("need at least one verification node")
	}

	// 4. CHECK: Enforce protocol compliance of collector cluster assignment
	//   (0) there is at least one collector cluster
	//   (a) assignment only contains nodes with collector role and positive weight
	//   (b) collectors have unique node IDs
	//   (c) each collector is assigned exactly to one cluster and is only listed once within that cluster
	//   (d) cluster contains at least one collector (i.e. is not empty)
	//   (e) cluster is composed of known nodes
	//   (f) cluster assignment lists the nodes in canonical ordering
	if len(setup.Assignments) == 0 { // enforce (0): at least one cluster
		return fmt.Errorf("need at least one collection cluster")
	}
	// Unpacking the cluster assignments (NodeIDs → IdentitySkeletons) enforces (a) - (f)
	_, err := factory.NewClusterList(setup.Assignments, activeParticipants.Filter(filter.HasRole[flow.IdentitySkeleton](flow.RoleCollection)))
	if err != nil {
		return fmt.Errorf("invalid cluster assignments: %w", err)
	}
	return nil
}

// IsValidExtendingEpochCommit checks whether an EpochCommit service event being added to the state is valid.
// In addition to intrinsic validity, we also check that it is valid w.r.t. the previous epoch setup event, and
// the current epoch status.
// CAUTION: This function assumes that all inputs besides extendingCommit are already validated.
// Expected errors during normal operations:
// * protocol.InvalidServiceEventError if the input service event is invalid to extend the currently active epoch
func IsValidExtendingEpochCommit(extendingCommit *flow.EpochCommit, protocolStateEntry *flow.ProtocolStateEntry, nextEpochSetupEvent *flow.EpochSetup) error {
	// The epoch setup event needs to happen before the commit.
	if protocolStateEntry.NextEpoch == nil {
		return NewInvalidServiceEventErrorf("missing epoch setup for epoch commit")
	}
	// Enforce EpochSetup is valid w.r.t to current epoch state
	if protocolStateEntry.NextEpoch.CommitID != flow.ZeroID { // We should only have a single epoch commit event per epoch.
		return NewInvalidServiceEventErrorf("duplicate epoch commit service event: %x", protocolStateEntry.NextEpoch.CommitID)
	}
	// Enforce the EpochSetup event is syntactically correct and compatible with the respective EpochSetup
	err := IsValidEpochCommit(extendingCommit, nextEpochSetupEvent)
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
