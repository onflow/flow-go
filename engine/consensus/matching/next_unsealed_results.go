package matching

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type nextUnsealedResults []*nextUnsealedResult

// This struct is made for debugging potential sealing halt
type nextUnsealedResult struct {
	BlockID                  flow.Identifier // the block of of the next unsealed block
	Height                   uint64          // the height of the block
	ResultID                 flow.Identifier // if we haven't received the result, it would be ZeroID
	IncorporatedResultID     flow.Identifier // to find seal in mempool
	TotalChunks              int
	FirstUnmatchedChunkIndex int  // show which chunk hasn't received approval
	CanBeSealed              bool // if true, then it should soon go to seals mempool
	SealedByEmergency        bool // if sealed by emergency since there are too many unsealed blocks
}

func (rs *nextUnsealedResults) String() string {
	bytes, err := json.Marshal(rs)
	if err != nil {
		return fmt.Sprintf("can not convert next unsealeds to json: %s", err.Error())
	}
	return string(bytes)
}
