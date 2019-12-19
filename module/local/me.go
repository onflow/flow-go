// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package local

import (
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Local struct {
	me flow.Identity
}

func New(id flow.Identity) (*Local, error) {
	l := &Local{
		me: id,
	}
	return l, nil
}

func (l *Local) NodeID() model.Identifier {
	return l.me.NodeID
}

func (l *Local) Address() string {
	return l.me.Address
}
