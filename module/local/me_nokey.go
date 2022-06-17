package local

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

type LocalNoKey struct {
	me *flow.Identity
}

func NewNoKey(id *flow.Identity) (*LocalNoKey, error) {
	l := &LocalNoKey{
		me: id,
	}
	return l, nil
}

func (l *LocalNoKey) NodeID() flow.Identifier {
	return l.me.NodeID
}

func (l *LocalNoKey) Address() string {
	return l.me.Address
}

func (l *LocalNoKey) Sign(msg []byte, hasher hash.Hasher) (crypto.Signature, error) {
	return nil, fmt.Errorf("no private key")
}

func (l *LocalNoKey) NotMeFilter() flow.IdentityFilter {
	return filter.Not(filter.HasNodeID(l.NodeID()))
}

// SignFunc provides a signature oracle that given a message, a hasher, and a signing function, it
// generates and returns a signature over the message using the node's private key
// as well as the input hasher by invoking the given signing function. The overall idea of this function
// is to not expose the private key to the caller.
func (l *LocalNoKey) SignFunc(data []byte, hasher hash.Hasher, f func(crypto.PrivateKey, []byte, hash.Hasher) (crypto.Signature,
	error)) (crypto.Signature, error) {
	return nil, fmt.Errorf("no private key to use for signing")
}
