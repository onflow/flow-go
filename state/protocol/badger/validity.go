package badger

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
)

// isValidExtendingEpochSetup checks whether an epoch setup service being
// added to the state is valid. In addition to intrinsic validitym, we also
// check that it is valid w.r.t. the previous epoch setup event, and the
// current epoch status.
func isValidExtendingEpochSetup(extendingSetup *flow.EpochSetup, activeSetup *flow.EpochSetup, status *flow.EpochStatus) error {

	// We should only have a single epoch setup event per epoch.
	if status.NextEpoch.SetupID != flow.ZeroID {
		// true iff EpochSetup event for NEXT epoch was already included before
		return protocol.NewInvalidServiceEventError("duplicate epoch setup service event: %x", status.NextEpoch.SetupID)
	}

	// The setup event should have the counter increased by one.
	if extendingSetup.Counter != activeSetup.Counter+1 {
		return protocol.NewInvalidServiceEventError("next epoch setup has invalid counter (%d => %d)", activeSetup.Counter, extendingSetup.Counter)
	}

	// The first view needs to be exactly one greater than the current epoch final view
	if extendingSetup.FirstView != activeSetup.FinalView+1 {
		return protocol.NewInvalidServiceEventError(
			"next epoch first view must be exactly 1 more than current epoch final view (%d != %d+1)",
			extendingSetup.FirstView,
			activeSetup.FinalView,
		)
	}

	// Finally, the epoch setup event must contain all necessary information.
	err := isValidEpochSetup(extendingSetup)
	if err != nil {
		return protocol.NewInvalidServiceEventError("invalid epoch setup: %w", err)
	}

	return nil
}

// isValidEpochSetup checks whether an epoch setup service event is intrinsically valid
func isValidEpochSetup(setup *flow.EpochSetup) error {
	return verifyEpochSetup(setup, true)
}

func verifyEpochSetup(setup *flow.EpochSetup, verifyNetworkAddress bool) error {
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

	// there should be no nodes with zero weight
	// TODO: we might want to remove the following as we generally want to allow nodes with
	// zero weight in the protocol state.
	for _, participant := range setup.Participants {
		if participant.Weight == 0 {
			return fmt.Errorf("node with zero weight (%x)", participant.NodeID)
		}
	}

	// the participants must be ordered by canonical order
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
	_, err := flow.NewClusterList(setup.Assignments, activeParticipants.Filter(filter.HasRole(flow.RoleCollection)))
	if err != nil {
		return fmt.Errorf("invalid cluster assignments: %w", err)
	}

	return nil
}

// isValidExtendingEpochCommit checks whether an epoch commit service being
// added to the state is valid. In addition to intrinsic validity, we also
// check that it is valid w.r.t. the previous epoch setup event, and the
// current epoch status.
func isValidExtendingEpochCommit(extendingCommit *flow.EpochCommit, extendingSetup *flow.EpochSetup, activeSetup *flow.EpochSetup, status *flow.EpochStatus) error {

	// We should only have a single epoch commit event per epoch.
	if status.NextEpoch.CommitID != flow.ZeroID {
		// true iff EpochCommit event for NEXT epoch was already included before
		return protocol.NewInvalidServiceEventError("duplicate epoch commit service event: %x", status.NextEpoch.CommitID)
	}

	// The epoch setup event needs to happen before the commit.
	if status.NextEpoch.SetupID == flow.ZeroID {
		return protocol.NewInvalidServiceEventError("missing epoch setup for epoch commit")
	}

	// The commit event should have the counter increased by one.
	if extendingCommit.Counter != activeSetup.Counter+1 {
		return protocol.NewInvalidServiceEventError("next epoch commit has invalid counter (%d => %d)", activeSetup.Counter, extendingCommit.Counter)
	}

	err := isValidEpochCommit(extendingCommit, extendingSetup)
	if err != nil {
		return state.NewInvalidExtensionErrorf("invalid epoch commit: %s", err)
	}

	return nil
}

// isValidEpochCommit checks whether an epoch commit service event is intrinsically valid.
func isValidEpochCommit(commit *flow.EpochCommit, setup *flow.EpochSetup) error {

	if len(setup.Assignments) != len(commit.ClusterQCs) {
		return fmt.Errorf("number of clusters (%d) does not number of QCs (%d)", len(setup.Assignments), len(commit.ClusterQCs))
	}

	if commit.Counter != setup.Counter {
		return fmt.Errorf("inconsistent epoch counter between commit (%d) and setup (%d) events in same epoch", commit.Counter, setup.Counter)
	}

	// make sure we have a valid DKG public key
	if commit.DKGGroupKey == nil {
		return fmt.Errorf("missing DKG public group key")
	}

	participants := setup.Participants.Filter(filter.IsValidDKGParticipant)
	if len(participants) != len(commit.DKGParticipantKeys) {
		return fmt.Errorf("participant list (len=%d) does not match dkg key list (len=%d)", len(participants), len(commit.DKGParticipantKeys))
	}

	return nil
}

// IsValidRootSnapshot checks internal consistency of root state snapshot
// if verifyResultID allows/disallows Result ID verification
func IsValidRootSnapshot(snap protocol.Snapshot, verifyResultID bool) error {

	segment, err := snap.SealingSegment()
	if err != nil {
		return fmt.Errorf("could not get sealing segment: %w", err)
	}
	result, seal, err := snap.SealedResult()
	if err != nil {
		return fmt.Errorf("could not latest sealed result: %w", err)
	}

	err = segment.Validate()
	if err != nil {
		return fmt.Errorf("invalid root sealing segment: %w", err)
	}

	highest := segment.Highest() // reference block of the snapshot
	lowest := segment.Lowest()   // last sealed block
	highestID := highest.ID()
	lowestID := lowest.ID()

	if result.BlockID != lowestID {
		return fmt.Errorf("root execution result for wrong block (%x != %x)", result.BlockID, lowest.ID())
	}

	if seal.BlockID != lowestID {
		return fmt.Errorf("root block seal for wrong block (%x != %x)", seal.BlockID, lowest.ID())
	}

	if verifyResultID {
		if seal.ResultID != result.ID() {
			return fmt.Errorf("root block seal for wrong execution result (%x != %x)", seal.ResultID, result.ID())
		}
	}

	// identities must be canonically ordered
	identities, err := snap.Identities(filter.Any)
	if err != nil {
		return fmt.Errorf("could not get identities for root snapshot: %w", err)
	}
	if !identities.Sorted(order.Canonical) {
		return fmt.Errorf("identities are not canonically ordered")
	}

	// root qc must be for reference block of snapshot
	qc, err := snap.QuorumCertificate()
	if err != nil {
		return fmt.Errorf("could not get qc for root snapshot: %w", err)
	}
	if qc.BlockID != highestID {
		return fmt.Errorf("qc is for wrong block (got: %x, expected: %x)", qc.BlockID, highestID)
	}

	firstView, err := snap.Epochs().Current().FirstView()
	if err != nil {
		return fmt.Errorf("could not get first view: %w", err)
	}
	finalView, err := snap.Epochs().Current().FinalView()
	if err != nil {
		return fmt.Errorf("could not get final view: %w", err)
	}

	// the segment must be fully within the current epoch
	if firstView > lowest.Header.View {
		return fmt.Errorf("lowest block of sealing segment has lower view than first view of epoch")
	}
	if highest.Header.View >= finalView {
		return fmt.Errorf("final view of epoch less than first block view")
	}

	return nil
}

// IsValidRootSnapshotQCs checks internal consistency of QCs that are included in the root state snapshot
// It verifies QCs for main consensus and for each collection cluster.
func IsValidRootSnapshotQCs(snap protocol.Snapshot) error {
	// validate main consensus QC
	err := validateRootQC(snap)
	if err != nil {
		return fmt.Errorf("invalid root QC: %w", err)
	}

	// validate each collection cluster separately
	curEpoch := snap.Epochs().Current()
	clusters, err := curEpoch.Clustering()
	if err != nil {
		return fmt.Errorf("could not get clustering for root snapshot: %w", err)
	}
	for clusterIndex := range clusters {
		cluster, err := curEpoch.Cluster(uint(clusterIndex))
		if err != nil {
			return fmt.Errorf("could not get cluster %d for root snapshot: %w", clusterIndex, err)
		}
		err = validateClusterQC(cluster)
		if err != nil {
			return fmt.Errorf("invalid cluster qc %d: %W", clusterIndex, err)
		}
	}
	return nil
}

// validateRootQC performs validation of root QC
// Returns nil on success
func validateRootQC(snap protocol.Snapshot) error {
	rootBlock, err := snap.Head()
	if err != nil {
		return fmt.Errorf("could not get root block: %w", err)
	}

	identities, err := snap.Identities(filter.IsVotingConsensusCommitteeMember)
	if err != nil {
		return fmt.Errorf("could not get root snapshot identities: %w", err)
	}

	rootQC, err := snap.QuorumCertificate()
	if err != nil {
		return fmt.Errorf("could not get root QC: %w", err)
	}

	dkg, err := snap.Epochs().Current().DKG()
	if err != nil {
		return fmt.Errorf("could not get DKG for root snapshot: %w", err)
	}

	hotstuffRootBlock := model.GenesisBlockFromFlow(rootBlock)
	committee, err := committees.NewStaticCommitteeWithDKG(identities, flow.Identifier{}, dkg)
	if err != nil {
		return fmt.Errorf("could not create static committee: %w", err)
	}
	verifier := verification.NewCombinedVerifier(committee, signature.NewConsensusSigDataPacker(committee))
	forks := &mocks.ForksReader{}
	hotstuffValidator := validator.New(committee, forks, verifier)
	err = hotstuffValidator.ValidateQC(rootQC, hotstuffRootBlock)
	if err != nil {
		return fmt.Errorf("could not validate root qc: %w", err)
	}
	return nil
}

// validateClusterQC performs QC validation of single collection cluster
// Returns nil on success
func validateClusterQC(cluster protocol.Cluster) error {
	clusterRootBlock := model.GenesisBlockFromFlow(cluster.RootBlock().Header)

	committee, err := committees.NewStaticCommittee(cluster.Members(), flow.Identifier{}, nil, nil)
	if err != nil {
		return fmt.Errorf("could not create static committee: %w", err)
	}
	verifier := verification.NewStakingVerifier()
	forks := &mocks.ForksReader{}
	hotstuffValidator := validator.New(committee, forks, verifier)
	err = hotstuffValidator.ValidateQC(cluster.RootQC(), clusterRootBlock)
	if err != nil {
		return fmt.Errorf("could not validate root qc: %w", err)
	}
	return nil
}
