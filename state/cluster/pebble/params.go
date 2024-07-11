package badger

import (
	"github.com/onflow/flow-go/model/flow"
)

type Params struct {
	state *State
}

func (p *Params) ChainID() flow.ChainID {
	return p.state.clusterID
}
