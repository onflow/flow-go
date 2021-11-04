package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol"
)

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

	// there should be no nodes with zero stake
	// TODO: we might want to remove the following as we generally want to allow nodes with
	// zero weight in the protocol state.
	for _, participant := range setup.Participants {
		if participant.Stake == 0 {
			return fmt.Errorf("node with zero stake (%x)", participant.NodeID)
		}
	}

	// the participants must be ordered by canonical order
	if !setup.Participants.Sorted(order.Canonical) {
		return fmt.Errorf("participants are not canonically ordered")
	}

	// STEP 3: sanity checks for individual roles
	// IMPORTANT: here we remove all nodes with zero weight, as they are allowed to partake
	// in communication but not in respective node functions
	activeParticipants := setup.Participants.Filter(filter.HasStake(true))

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

// isValidRootSnapshot checks internal consistency of root state snapshot
// if verifyResultID allows/disallows Result ID verification
func isValidRootSnapshot(snap protocol.Snapshot, verifyResultID bool) error {

	segment, err := snap.SealingSegment()
	if err != nil {
		return fmt.Errorf("could not get sealing segment: %w", err)
	}
	result, seal, err := snap.SealedResult()
	if err != nil {
		return fmt.Errorf("could not latest sealed result: %w", err)
	}

	if len(segment.Blocks) == 0 {
		return fmt.Errorf("invalid empty sealing segment")
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
		return fmt.Errorf("qc is for wrong block (got: %x, expected: %x)", qc.BlockID, highest.ID())
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
		return fmt.Errorf("tail block of sealing segment has lower view than first view of epoch")
	}
	if highest.Header.View >= finalView {
		return fmt.Errorf("final view of epoch less than first block view")
	}

	return nil
}
