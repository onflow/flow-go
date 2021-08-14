// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package local

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

type Local struct {
	me *flow.Identity
	sk crypto.PrivateKey // instance of the node's private staking key
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

// SignFunc provides a signature oracle that given a message, a hasher, and a signing function, it
// generates and returns a signature over the message using the node's private key
// as well as the input hasher by invoking the given signing function. The overall idea of this function
// is to not expose the private key to the caller.
func (l *Local) SignFunc(data []byte, hasher hash.Hasher, f func(crypto.PrivateKey, []byte, hash.Hasher) (crypto.Signature,
	error)) (crypto.Signature, error) {
	return f(l.sk, data, hasher)
}
