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
	for _, identity := range setup.Identities {
		_, ok := identLookup[identity.NodeID]
		if ok {
			return fmt.Errorf("duplicate node identifier (%x)", identity.NodeID)
		}
		identLookup[identity.NodeID] = struct{}{}
	}

	// there should be no duplicate node addresses
	addrLookup := make(map[string]struct{})
	for _, identity := range setup.Identities {
		_, ok := addrLookup[identity.Address]
		if ok {
			return fmt.Errorf("duplicate node address (%x)", identity.Address)
		}
		addrLookup[identity.Address] = struct{}{}
	}

	// there should be no nodes with zero stake
	for _, identity := range setup.Identities {
		if identity.Stake == 0 {
			return fmt.Errorf("node with zero stake (%x)", identity.NodeID)
		}
	}

	// we need at least one node of each role
	roles := make(map[flow.Role]uint)
	for _, identity := range setup.Identities {
		roles[identity.Role]++
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
	_, err := flow.NewClusterList(setup.Assignments, setup.Identities.Filter(filter.HasRole(flow.RoleCollection)))
	if err != nil {
		return fmt.Errorf("invalid cluster assignments: %w", err)
	}

	// the seed needs to be at least minimum length
	if len(setup.Seed) < crypto.MinSeedLength {
		return fmt.Errorf("seed has insufficient length (%d < %d)", len(setup.Seed), crypto.MinSeedLength)
	}

	return nil
}

func validCommit(setup *flow.EpochCommit) error {

	return nil
}
