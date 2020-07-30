package epoch

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Setup struct {
	Counter      uint64
	FinalView    uint64
	Participants flow.IdentityList
	Assignments  flow.AssignmentList
	Seed         []byte
}
