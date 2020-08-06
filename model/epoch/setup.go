package epoch

import (
	"encoding/binary"

	"github.com/dapperlabs/flow-go/model/flow"
)

// TODO docs
type Setup struct {
	Counter      uint64
	FinalView    uint64
	Participants flow.IdentityList
	Assignments  flow.AssignmentList
	Seed         []byte
}

func (setup *Setup) ToServiceEvent() *ServiceEvent {
	return &ServiceEvent{
		Type:  ServiceEventSetup,
		Event: setup,
	}
}

// ID returns a unique ID for the epoch, based on the counter. This
// is used as a work-around for the current caching layer, which only
// supports flow entities keyed by ID for now.
func (setup *Setup) ID() flow.Identifier {
	var commitID flow.Identifier
	binary.LittleEndian.PutUint64(commitID[:], setup.Counter)
	return commitID
}
