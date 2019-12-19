package gossip

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// PeersTable represents a table that stores the mapping
// between the IP of a node and its ID
type PeersTable interface {

	// Add adds a new mapping to the peers table
	Add(ID flow.Identifier, IP string)

	// GetID returns the ID corresponding to the given IP
	GetID(IP string) (flow.Identifier, error)

	// GetIP returns the IP corresponding to the given ID
	GetIP(ID flow.Identifier) (string, error)

	// GetIPs receives a group of IDs and returns their corresponding IPs
	GetIPs(IDs ...flow.Identifier) ([]string, error)

	// GetIDs receives a group of IPs and returns their corresponding IDs
	GetIDs(IPs ...string) ([]flow.Identifier, error)
}
