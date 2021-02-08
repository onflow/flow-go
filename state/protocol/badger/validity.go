package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

func isValidEpochSetup(setup *flow.EpochSetup) error {
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

	// there should be no duplicate node addresses
	addrLookup := make(map[string]struct{})
	for _, participant := range setup.Participants {
		_, ok := addrLookup[participant.Address]
		if ok {
			return fmt.Errorf("duplicate node address (%x)", participant.Address)
		}
		addrLookup[participant.Address] = struct{}{}
	}

	// there should be no nodes with zero stake
	// TODO: we might want to remove the following as we generally want to allow nodes with
	// zero weight in the protocol state.
	for _, participant := range setup.Participants {
		if participant.Stake == 0 {
			return fmt.Errorf("node with zero stake (%x)", participant.NodeID)
		}
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

	participants := setup.Participants.Filter(filter.HasRole(flow.RoleConsensus))

	// make sure each participant of the epoch has a DKG entry
	for _, participant := range participants {
		_, exists := commit.DKGParticipants[participant.NodeID]
		if !exists {
			return fmt.Errorf("missing DKG participant data (%x)", participant.NodeID)
		}
	}

	// make sure that there is no extra data
	if len(participants) != len(commit.DKGParticipants) {
		return fmt.Errorf("DKG data contains extra entries")
	}

	return nil
}

// isValidRootSnapshot checks internal consistency of root state snapshot
func isValidRootSnapshot(snap protocol.Snapshot) error {

	segment, err := snap.SealingSegment()
	if err != nil {
		return fmt.Errorf("could not get sealing segment: %w", err)
	}
	seal, err := snap.LatestSeal()
	if err != nil {
		return fmt.Errorf("could not latest seal: %w", err)
	}
	result, err := snap.LatestResult()
	if err != nil {
		return fmt.Errorf("could not get latest result: %w", err)
	}

	if len(segment) == 0 {
		return fmt.Errorf("invalid empty sealing segment")
	}
	head := segment[len(segment)-1] // reference block of the snapshot
	tail := segment[0]              // last sealed block

	if result.BlockID != tail.ID() {
		return fmt.Errorf("root execution result for wrong block (%x != %x)", result.BlockID, tail.ID())
	}

	if seal.BlockID != tail.ID() {
		return fmt.Errorf("root block seal for wrong block (%x != %x)", seal.BlockID, tail.ID())
	}

	if seal.ResultID != result.ID() {
		return fmt.Errorf("root block seal for wrong execution result (%x != %x)", seal.ResultID, result.ID())
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
	if firstView > tail.Header.View {
		return fmt.Errorf("root block has lower view than first view of epoch")
	}
	if head.Header.View >= finalView {
		return fmt.Errorf("final view of epoch less than first block view")
	}

	return nil
}
