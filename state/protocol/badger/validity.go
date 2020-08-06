package badger

import (
	"fmt"

	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
)

func validSetup(setup *flow.EpochSetup) error {

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
	for _, participant := range setup.Participants {
		if participant.Stake == 0 {
			return fmt.Errorf("node with zero stake (%x)", participant.NodeID)
		}
	}

	// we need at least one node of each role
	roles := make(map[flow.Role]uint)
	for _, participant := range setup.Participants {
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

	// we need at least one collection cluster
	if len(setup.Assignments) == 0 {
		return fmt.Errorf("need at least one collection cluster")
	}

	// the collection cluster assignments need to be valid
	_, err := flow.NewClusterList(setup.Assignments, setup.Participants.Filter(filter.HasRole(flow.RoleCollection)))
	if err != nil {
		return fmt.Errorf("invalid cluster assignments: %w", err)
	}

	// the seed needs to be at least minimum length
	if len(setup.Seed) < crypto.MinSeedLength {
		return fmt.Errorf("seed has insufficient length (%d < %d)", len(setup.Seed), crypto.MinSeedLength)
	}

	return nil
}

func validCommit(commit *flow.EpochCommit, participants flow.IdentityList) error {

	// make sure we have a valid DKG public key
	if commit.DKGGroupKey == nil {
		return fmt.Errorf("missing DKG public group key")
	}

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
