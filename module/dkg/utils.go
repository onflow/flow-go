package dkg

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

func GetDKGCommitteeIndex(nodeID flow.Identifier, participants flow.IdentityList) (int, error) {
	myIndex := -1
	for i, id := range participants.NodeIDs() {
		if id == nodeID {
			myIndex = i
			break
		}
	}
	if myIndex < 0 {
		return myIndex, fmt.Errorf("node does not belong to dkg committee")
	}

	return myIndex, nil
}
