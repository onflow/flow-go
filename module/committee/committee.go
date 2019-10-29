// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package committee

import (
	"regexp"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

var (
	idParser = regexp.MustCompile(`^(\w+)-(\w+)@([\w\.]+:\d{1,5})$`)
)

// Committee represents the identity table for staked nodes in the flow system.
type Committee struct {
	me         flow.Identity
	identities map[string]flow.Identity
}

// EntryToFields takes the a committee entry and returns the parsed node
// info fields
//
// > EntryToFields("consensus-consensus1@localhost:7297")
// "consensus", "consensus1", "localhost:7297", nil
func EntryToFields(entry string) (string, string, string, error) {
	// try to parse the nodes
	fields := idParser.FindStringSubmatch(entry)
	if fields == nil {
		return "", "", "", errors.Errorf("invalid node entry (%s)", entry)
	}

	role := fields[1]
	id := fields[2]
	address := fields[3]
	return role, id, address, nil
}

// EntryToID takes the a committee entry and returns the node identity.
func EntryToID(entry string) (string, error) {
	_, id, _, err := EntryToFields(entry)
	return id, err
}

// New generates a new committee of node identities.
func New(entries []string, identity string) (*Committee, error) {

	// create committee with identity table
	c := &Committee{
		identities: make(map[string]flow.Identity),
	}

	// try to parse the nodes
	for _, entry := range entries {
		// try to parse the expression
		role, id, address, err := EntryToFields(entry)
		if err != nil {
			return nil, errors.Errorf("invalid node entry (%s)", entry)
		}

		// check for duplicates
		_, ok := c.identities[id]
		if ok {
			return nil, errors.Errorf("duplicate node identity (%s)", id)
		}

		// add entry to identity table
		node := flow.Identity{
			NodeID:  id,
			Role:    role,
			Address: address,
		}
		c.identities[id] = node
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
