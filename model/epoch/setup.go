package epoch

import (
	"encoding/binary"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Setup struct {
	Counter      uint64
	FinalView    uint64
	Participants flow.IdentityList
	Assignments  flow.AssignmentList
	Seed         []byte
}

// ID returns a unique ID for the epoch, based on the counter. This
// is used as a work-around for the current caching layer, which only
// suports flow entities keyed by ID for now.
func (s *Setup) ID() flow.Identifier {
	var commitID flow.Identifier
	binary.LittleEndian.PutUint64(commitID[:], s.Counter)
	return commitID
}
