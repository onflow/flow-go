// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package local

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
)

type Local struct {
	me *flow.Identity
	sk crypto.PrivateKey // instance of the node's private key
}

func New(id *flow.Identity, sk crypto.PrivateKey) (*Local, error) {
	l := &Local{
		me: id,
		sk: sk,
	}
	return l, nil
}

func (l *Local) NodeID() flow.Identifier {
	return l.me.NodeID
}

func (l *Local) Address() string {
	return l.me.Address
}

func (l *Local) Sign(msg []byte, hasher hash.Hasher) (crypto.Signature, error) {
	return l.sk.Sign(msg, hasher)
}

func (l *Local) NotMeFilter() flow.IdentityFilter {
	return filter.Not(filter.HasNodeID(l.NodeID()))
}

func (l *Local) StakingKey() crypto.PrivateKey {
	return l.sk
}
