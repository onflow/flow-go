// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package committee

import (
	"regexp"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// Committee represents the identity table for staked nodes in the flow system.
type Committee struct {
	me         flow.Identity
	identities map[string]flow.Identity
}

// New generates a new committee of node identities.
func New(entries []string, identity string) (*Committee, error) {

	// create committee with identity table
	c := &Committee{
		identities: make(map[string]flow.Identity),
	}

	// try to parse the node identities
	rx := regexp.MustCompile(`^(\w+)-(\w+)@([\w\.]+:\d{1,5})$`)
	for _, entry := range entries {

		// try to parse the expression
		fields := rx.FindStringSubmatch(entry)
		if fields == nil {
			return nil, errors.Errorf("invalid node entry (%s)", entry)
		}

		// check for duplicates
		nodeID := fields[2]
		_, ok := c.identities[nodeID]
		if ok {
			return nil, errors.Errorf("duplicate node identity (%s)", nodeID)
		}

		// add entry to identity table
		identity := flow.Identity{
			NodeID:  fields[2],
			Role:    fields[1],
			Address: fields[3],
		}
		c.identities[nodeID] = identity
	}

	// check if we are present in the node identity table
	me, ok := c.identities[identity]
	if !ok {
		return nil, errors.Errorf("entry for own identity missing (%s)", identity)
	}
	c.me = me

	return c, nil
}

// Me returns our own node identity.
func (c *Committee) Me() flow.Identity {
	return c.me
}

// Get will return the node with the given ID.
func (c *Committee) Get(id string) (flow.Identity, error) {
	node, ok := c.identities[id]
	if !ok {
		return flow.Identity{}, errors.New("node not found")
	}
	return node, nil
}

// Select will returns the identities fulfilling the given filters.
func (c *Committee) Select(filters ...module.IdentityFilter) (flow.IdentityList, error) {
	var identities flow.IdentityList
Outer:
	for _, node := range c.identities {
		for _, filter := range filters {
			if !filter(node) {
				continue Outer
			}
		}
		identities = append(identities, node)
	}
	return identities, nil
}
