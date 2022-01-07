package invalid

import (
	"github.com/onflow/flow-go/model/flow"
)

// Params represents parameters for an invalid state snapshot query.
type Params struct {
	err error
}

func (p *Params) ChainID() (flow.ChainID, error) {
	return "", p.err
}

func (p *Params) SporkID() (flow.Identifier, error) {
	return flow.ZeroID, p.err
}

func (p *Params) ProtocolVersion() (uint, error) {
	return 0, p.err
}
