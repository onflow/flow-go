package invalid

import (
	"github.com/onflow/flow-go/model/flow"
)

// Params represents parameters for an invalid state snapshot query.
type Params struct {
	err error
}

func (p Params) ChainID() flow.ChainID {
	return ""
}

func (p Params) SporkID() flow.Identifier {
	return flow.ZeroID
}

func (p Params) SporkRootBlockHeight() uint64 {
	return 0
}

func (p Params) ProtocolVersion() uint {
	return 0
}

func (p Params) EpochCommitSafetyThreshold() uint64 {
	return 0
}
