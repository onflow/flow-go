package peerstable

import (
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/flow"
)

// PeersTable is a type that keeps a mapping from IP to an ID and vice versa.
type PeersTable struct {
	fromIDToIP map[flow.Identifier]string
	fromIPToID map[string]flow.Identifier
}

// NewPeersTable returns a new instance of PeersTable
func NewPeersTable() (*PeersTable, error) {
	return &PeersTable{
		fromIDToIP: make(map[flow.Identifier]string),
		fromIPToID: make(map[string]flow.Identifier),
	}, nil
}

// Add adds a new mapping to the peers table
func (pt *PeersTable) Add(ID flow.Identifier, IP string) {
	pt.fromIDToIP[ID] = IP
	pt.fromIPToID[IP] = ID
}

// GetID receives an IP and returns its corresponding ID
func (pt *PeersTable) GetID(IP string) (flow.Identifier, error) {
	ID, ok := pt.fromIPToID[IP]
	if !ok {
		return flow.Identifier{}, errors.Errorf("could not find ID linked with IP (%v)", IP)
	}

	return ID, nil
}

// GetIP receives a ID and returns its corresponding IP
func (pt *PeersTable) GetIP(ID flow.Identifier) (string, error) {
	IP, ok := pt.fromIDToIP[ID]

	if !ok {
		return "", errors.Errorf("could not find IP linked with ID (%v)", ID)
	}

	return IP, nil
}

// GetIPs receives a group of IDs and returns their corresponding IPs
func (pt *PeersTable) GetIPs(IDs ...flow.Identifier) ([]string, error) {
	IPs := make([]string, len(IDs))

	for i, ID := range IDs {
		IP, err := pt.GetIP(ID)
		if err != nil {
			return nil, errors.Wrap(err, "could not find all IPs")
		}

		IPs[i] = IP
	}

	return IPs, nil
}

// GetIDs receives a group of IPs and returns their corresponding IDs
func (pt *PeersTable) GetIDs(IPs ...string) ([]flow.Identifier, error) {

	IDs := make([]flow.Identifier, len(IPs))

	for i, IP := range IPs {
		ID, err := pt.GetID(IP)
		if err != nil {
			return nil, errors.Wrap(err, "could not find all IDs")
		}

		IDs[i] = ID
	}

	return IDs, nil
}
