package badger

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(genesis *cluster.Block) error {
	panic("TODO")
}

func (m *Mutator) Extend(blockID flow.Identifier) error {
	panic("TODO")
}
