// +build relic

package signature

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/module"
)

type SingleSigner struct {
	hasher hash.Hasher
	local  module.Local
}

func NewSingleSigner(tag string, local module.Local) *SingleSigner {
	return &SingleSigner{
		hasher: crypto.NewBLSKMAC(tag),
		local:  local,
	}
}

func (s *SingleSigner) Sign(msg []byte) (crypto.Signature, error) {
	return s.local.Sign(msg, s.hasher)
}
