// +build !relic

package signature

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/module"
)

type DKGVerifier struct {
	hasher hash.Hasher
}

func NewDKGVerifier(tag string) *DKGVerifier {
	return &DKGVerifier{
		hasher: crypto.NewBLSKMAC(tag),
	}
}

func (v *DKGVerifier) Verify(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error) {
	return key.Verify(sig, msg, v.hasher)
}

type DKGSigner struct {
	*DKGVerifier
	privKey crypto.PrivateKey
}

func NewDKGSigner(tag string, privKey encodable.RandomBeaconPrivKey) module.Signer {
	return &DKGSigner{
		DKGVerifier: NewDKGVerifier(tag),
		privKey:     privKey,
	}
}

func (s *DKGSigner) Sign(msg []byte) (crypto.Signature, error) {
	return s.privKey.Sign(msg, s.hasher)
}
