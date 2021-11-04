// +build relic

package signature

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/module"
)

type RandomBeaconSigner struct {
	hasher hash.Hasher
	priv   crypto.PrivateKey
}

func NewRandomBeaconSigner(tag string, priv crypto.PrivateKey) *RandomBeaconSigner {
	return &RandomBeaconSigner{
		hasher: crypto.NewBLSKMAC(tag),
		priv:   priv,
	}
}

// Sign will use the internal private key share to generate a threshold signature
// share.
func (s *RandomBeaconSigner) Sign(msg []byte) (crypto.Signature, error) {
	return s.priv.Sign(msg, s.hasher)
}

type StakingSigner struct {
	hasher hash.Hasher
	local  module.Local
}

func NewStakingSigner(tag string, local module.Local) *StakingSigner {
	return &StakingSigner{
		hasher: crypto.NewBLSKMAC(tag),
		local:  local,
	}
}

// Sign will sign the given message bytes with the internal private key and
// return the signature on success.
func (s *StakingSigner) Sign(msg []byte) (crypto.Signature, error) {
	return s.local.Sign(msg, s.hasher)
}
