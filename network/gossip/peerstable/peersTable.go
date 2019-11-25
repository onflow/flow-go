package peerstable

import (
	"github.com/pkg/errors"
)

// PeersTable is a type that keeps a mapping from IP to an ID and vice versa.
type PeersTable struct {
	fromIDToIP map[string]string
	fromIPToID map[string]string
}

// NewPeersTable returns a new instance of PeersTable
func NewPeersTable() (*PeersTable, error) {
	return &PeersTable{
		fromIDToIP: make(map[string]string),
		fromIPToID: make(map[string]string),
	}, nil
}

// Add adds a new mapping to the peers table
func (pt *PeersTable) Add(ID string, IP string) {
	pt.fromIDToIP[ID] = IP
	pt.fromIPToID[IP] = ID
}

// GetID receives an IP and returns its corresponding ID
func (pt *PeersTable) GetID(IP string) (string, error) {
	ID, ok := pt.fromIPToID[IP]
	if !ok {
		return "", errors.Errorf("could not find ID linked with IP (%v)", IP)
	}

	return ID, nil
}

// GetIP receives a ID and returns its corresponding IP
func (pt *PeersTable) GetIP(ID string) (string, error) {
	IP, ok := pt.fromIDToIP[ID]

	if !ok {
		return "", errors.Errorf("could not find IP linked with ID (%v)", ID)
	}

	return IP, nil
}

// GetIPs receives a group of IDs and returns their corresponding IPs
func (pt *PeersTable) GetIPs(IDs ...string) ([]string, error) {
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
func (pt *PeersTable) GetIDs(IPs ...string) ([]string, error) {

	IDs := make([]string, len(IPs))

	for i, IP := range IPs {
		ID, err := pt.GetID(IP)
		if err != nil {
			return nil, errors.Wrap(err, "could not find all IDs")
		}

		IDs[i] = ID
	}

	return IDs, nil
}
