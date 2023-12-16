package badger

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol"
)

// isValidExtendingEpochSetup checks whether an epoch setup service being
// added to the state is valid. In addition to intrinsic validity, we also
// check that it is valid w.r.t. the previous epoch setup event, and the
// current epoch status.
// Assumes all inputs besides extendingSetup are already validated.
// Expected errors during normal operations:
// * protocol.InvalidServiceEventError if the input service event is invalid to extend the currently active epoch status
func isValidExtendingEpochSetup(extendingSetup *flow.EpochSetup, activeSetup *flow.EpochSetup, status *flow.EpochStatus) error {
	// We should only have a single epoch setup event per epoch.
	if status.NextEpoch.SetupID != flow.ZeroID {
		// true iff EpochSetup event for NEXT epoch was already included before
		return protocol.NewInvalidServiceEventErrorf("duplicate epoch setup service event: %x", status.NextEpoch.SetupID)
	}

	// The setup event should have the counter increased by one.
	if extendingSetup.Counter != activeSetup.Counter+1 {
		return protocol.NewInvalidServiceEventErrorf("next epoch setup has invalid counter (%d => %d)", activeSetup.Counter, extendingSetup.Counter)
	}

	// The first view needs to be exactly one greater than the current epoch final view
	if extendingSetup.FirstView != activeSetup.FinalView+1 {
		return protocol.NewInvalidServiceEventErrorf(
			"next epoch first view must be exactly 1 more than current epoch final view (%d != %d+1)",
			extendingSetup.FirstView,
			activeSetup.FinalView,
		)
	}

	// Finally, the epoch setup event must contain all necessary information.
	err := verifyEpochSetup(extendingSetup, true)
	if err != nil {
		return protocol.NewInvalidServiceEventErrorf("invalid epoch setup: %w", err)
	}

	return nil
}

// verifyEpochSetup checks whether an `EpochSetup` event is syntactically correct.
// The boolean parameter `verifyNetworkAddress` controls, whether we want to permit
// nodes to share a networking address.
// This is a side-effect-free function. Any error return indicates that the
// EpochSetup event is not compliant with protocol rules.
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

	// the participants must be listed in canonical order
	if !order.IdentityListCanonical(setup.Participants) {
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

// isValidExtendingEpochCommit checks whether an epoch commit service being
// added to the state is valid. In addition to intrinsic validity, we also
// check that it is valid w.r.t. the previous epoch setup event, and the
// current epoch status.
// Assumes all inputs besides extendingCommit are already validated.
// Expected errors during normal operations:
// * protocol.InvalidServiceEventError if the input service event is invalid to extend the currently active epoch status
func isValidExtendingEpochCommit(extendingCommit *flow.EpochCommit, extendingSetup *flow.EpochSetup, activeSetup *flow.EpochSetup, status *flow.EpochStatus) error {

	// We should only have a single epoch commit event per epoch.
	if status.NextEpoch.CommitID != flow.ZeroID {
		// true iff EpochCommit event for NEXT epoch was already included before
		return protocol.NewInvalidServiceEventErrorf("duplicate epoch commit service event: %x", status.NextEpoch.CommitID)
	}

	// The epoch setup event needs to happen before the commit.
	if status.NextEpoch.SetupID == flow.ZeroID {
		return protocol.NewInvalidServiceEventErrorf("missing epoch setup for epoch commit")
	}

	// The commit event should have the counter increased by one.
	if extendingCommit.Counter != activeSetup.Counter+1 {
		return protocol.NewInvalidServiceEventErrorf("next epoch commit has invalid counter (%d => %d)", activeSetup.Counter, extendingCommit.Counter)
	}

	err := isValidEpochCommit(extendingCommit, extendingSetup)
	if err != nil {
		return protocol.NewInvalidServiceEventErrorf("invalid epoch commit: %s", err)
	}

	return nil
}

// isValidEpochCommit checks whether an epoch commit service event is intrinsically valid.
// Assumes the input flow.EpochSetup event has already been validated.
// Expected errors during normal operations:
// * protocol.InvalidServiceEventError if the EpochCommit is invalid
func isValidEpochCommit(commit *flow.EpochCommit, setup *flow.EpochSetup) error {

	if len(setup.Assignments) != len(commit.ClusterQCs) {
		return protocol.NewInvalidServiceEventErrorf("number of clusters (%d) does not number of QCs (%d)", len(setup.Assignments), len(commit.ClusterQCs))
	}

	if commit.Counter != setup.Counter {
		return protocol.NewInvalidServiceEventErrorf("inconsistent epoch counter between commit (%d) and setup (%d) events in same epoch", commit.Counter, setup.Counter)
	}

	// make sure we have a valid DKG public key
	if commit.DKGGroupKey == nil {
		return protocol.NewInvalidServiceEventErrorf("missing DKG public group key")
	}

	participants := setup.Participants.Filter(filter.IsValidDKGParticipant)
	if len(participants) != len(commit.DKGParticipantKeys) {
		return protocol.NewInvalidServiceEventErrorf("participant list (len=%d) does not match dkg key list (len=%d)", len(participants), len(commit.DKGParticipantKeys))
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
	lowest := segment.Sealed()   // last sealed block
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
	if !order.IdentityListCanonical(identities) {
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

	err = validateVersionBeacon(snap)
	if err != nil {
		return err
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
			return fmt.Errorf("invalid cluster qc %d: %w", clusterIndex, err)
		}
	}
	return nil
}

// validateRootQC performs validation of root QC
// Returns nil on success
func validateRootQC(snap protocol.Snapshot) error {
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

	committee, err := committees.NewStaticCommitteeWithDKG(identities, flow.Identifier{}, dkg)
	if err != nil {
		return fmt.Errorf("could not create static committee: %w", err)
	}
	verifier := verification.NewCombinedVerifier(committee, signature.NewConsensusSigDataPacker(committee))
	hotstuffValidator := validator.New(committee, verifier)
	err = hotstuffValidator.ValidateQC(rootQC)
	if err != nil {
		return fmt.Errorf("could not validate root qc: %w", err)
	}
	return nil
}

// validateClusterQC performs QC validation of single collection cluster
// Returns nil on success
func validateClusterQC(cluster protocol.Cluster) error {
	committee, err := committees.NewStaticCommittee(cluster.Members(), flow.Identifier{}, nil, nil)
	if err != nil {
		return fmt.Errorf("could not create static committee: %w", err)
	}
	verifier := verification.NewStakingVerifier()
	hotstuffValidator := validator.New(committee, verifier)
	err = hotstuffValidator.ValidateQC(cluster.RootQC())
	if err != nil {
		return fmt.Errorf("could not validate root qc: %w", err)
	}
	return nil
}

// validateVersionBeacon returns an InvalidServiceEventError if the snapshot
// version beacon is invalid
func validateVersionBeacon(snap protocol.Snapshot) error {
	errf := func(msg string, args ...any) error {
		return protocol.NewInvalidServiceEventErrorf(msg, args)
	}

	versionBeacon, err := snap.VersionBeacon()
	if err != nil {
		return errf("could not get version beacon: %w", err)
	}

	if versionBeacon == nil {
		return nil
	}

	head, err := snap.Head()
	if err != nil {
		return errf("could not get snapshot head: %w", err)
	}

	// version beacon must be included in a past block to be effective
	if versionBeacon.SealHeight > head.Height {
		return errf("version table height higher than highest height")
	}

	err = versionBeacon.Validate()
	if err != nil {
		return errf("version beacon is invalid: %w", err)
	}

	return nil
}

// ValidRootSnapshotContainsEntityExpiryRange performs a sanity check to make sure the
// root snapshot has enough history to encompass at least one full entity expiry window.
// Entities (in particular transactions and collections) may reference a block within
// the past `flow.DefaultTransactionExpiry` blocks, so a new node must begin with at least
// this many blocks worth of history leading up to the snapshot's root block.
//
// Currently, Access Nodes and Consensus Nodes require root snapshots passing this validator function.
//
//   - Consensus Nodes because they process guarantees referencing past blocks
//   - Access Nodes because they index transactions referencing past blocks
//
// One of the following conditions must be satisfied to pass this validation:
//  1. This is a snapshot build from a first block of spork
//     -> there is no earlier history which transactions/collections could reference
//  2. This snapshot sealing segment contains at least one expiry window of blocks
//     -> all possible reference blocks in future transactions/collections will be within the initial history.
//  3. This snapshot sealing segment includes the spork root block
//     -> there is no earlier history which transactions/collections could reference
func ValidRootSnapshotContainsEntityExpiryRange(snapshot protocol.Snapshot) error {
	isSporkRootSnapshot, err := protocol.IsSporkRootSnapshot(snapshot)
	if err != nil {
		return fmt.Errorf("could not check if root snapshot is a spork root snapshot: %w", err)
	}
	// Condition 1 satisfied
	if isSporkRootSnapshot {
		return nil
	}

	head, err := snapshot.Head()
	if err != nil {
		return fmt.Errorf("could not query root snapshot head: %w", err)
	}

	sporkRootBlockHeight, err := snapshot.Params().SporkRootBlockHeight()
	if err != nil {
		return fmt.Errorf("could not query spork root block height: %w", err)
	}

	sealingSegment, err := snapshot.SealingSegment()
	if err != nil {
		return fmt.Errorf("could not query sealing segment: %w", err)
	}

	sealingSegmentLength := uint64(len(sealingSegment.AllBlocks()))
	transactionExpiry := uint64(flow.DefaultTransactionExpiry)
	blocksInSpork := head.Height - sporkRootBlockHeight + 1 // range is inclusive on both ends

	// Condition 3:
	// check if head.Height - sporkRootBlockHeight < flow.DefaultTransactionExpiry
	// this is the case where we bootstrap early into the spork and there is simply not enough blocks
	if blocksInSpork < transactionExpiry {
		// the distance to spork root is less than transaction expiry, we need all blocks back to the spork root.
		if sealingSegmentLength != blocksInSpork {
			return fmt.Errorf("invalid root snapshot length, expecting exactly (%d), got (%d)", blocksInSpork, sealingSegmentLength)
		}
	} else {
		// Condition 2:
		// the distance to spork root is more than transaction expiry, we need at least `transactionExpiry` many blocks
		if sealingSegmentLength < transactionExpiry {
			return fmt.Errorf("invalid root snapshot length, expecting at least (%d), got (%d)",
				transactionExpiry, sealingSegmentLength)
		}
	}
	return nil
}
