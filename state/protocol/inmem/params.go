package inmem

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type Params struct {
	enc EncodableParams
}

var _ protocol.GlobalParams = (*Params)(nil)

func NewParams(enc EncodableParams) *Params {
	return &Params{
		enc: enc,
	}
}

func (p Params) ChainID() flow.ChainID {
	return p.enc.ChainID
}

func (p Params) SporkID() flow.Identifier {
	return p.enc.SporkID
}

func (p Params) SporkRootBlockHeight() uint64 {
	return p.enc.SporkRootBlockHeight
}

func (p Params) ProtocolVersion() uint {
	return p.enc.ProtocolVersion
}

func (p Params) EpochCommitSafetyThreshold() uint64 {
	return p.enc.EpochCommitSafetyThreshold
}
