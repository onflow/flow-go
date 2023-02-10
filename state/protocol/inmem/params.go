package inmem

import (
	"github.com/onflow/flow-go/model/flow"
)

type Params struct {
	enc EncodableParams
}

func (p Params) ChainID() (flow.ChainID, error) {
	return p.enc.ChainID, nil
}

func (p Params) SporkID() (flow.Identifier, error) {
	return p.enc.SporkID, nil
}

func (p Params) SporkRootBlockHeight() (uint64, error) {
	return p.enc.SporkRootBlockHeight, nil
}

func (p Params) ProtocolVersion() (uint, error) {
	return p.enc.ProtocolVersion, nil
}

func (p Params) EpochCommitSafetyThreshold() (uint64, error) {
	return p.enc.EpochCommitSafetyThreshold, nil
}
