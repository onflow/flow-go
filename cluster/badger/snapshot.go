package badger

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Snapshot struct {
	state   *State
	blockID flow.Identifier
	final   bool
}

func (s *Snapshot) Collection() (*flow.Collection, error) {
	_ = s.state
	_ = s.blockID
	_ = s.final
	panic("TODO")
}
