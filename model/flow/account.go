// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
)

// Account represents an account on the Flow network.
//
// An account can be an externally owned account or a contract account with code.
type Account struct {
	Address Address
	Balance uint64
	Code    []byte
	Keys    []AccountPublicKey
}

// AccountPublicKey is a public key associated with an account.
//
// An account public key contains the public key, signing and hashing algorithms, and a key weight.
type AccountPublicKey struct {
	PublicKey crypto.PublicKey
	SignAlgo  crypto.SigningAlgorithm
	HashAlgo  hash.HashingAlgorithm
	SeqNumber uint64
	Weight    int
}

// Validate returns an error if this account key is invalid.
//
// An account key can be invalid for the following reasons:
// - It specifies an incompatible signature/hash algorithm pairing
// - (TODO) It specifies a negative key weight
func (a AccountPublicKey) Validate() error {
	if !CompatibleAlgorithms(a.SignAlgo, a.HashAlgo) {
		return errors.Errorf(
			"signing algorithm (%s) is incompatible with hashing algorithm (%s)",
			a.SignAlgo,
			a.HashAlgo,
		)
	}
	return nil
}

// AccountPrivateKey is a private key associated with an account.
type AccountPrivateKey struct {
	PrivateKey crypto.PrivateKey
	SignAlgo   crypto.SigningAlgorithm
	HashAlgo   hash.HashingAlgorithm
}

// PublicKey returns a weighted public key.
func (a AccountPrivateKey) PublicKey(weight int) AccountPublicKey {
	return AccountPublicKey{
		PublicKey: a.PrivateKey.PublicKey(),
		SignAlgo:  a.SignAlgo,
		HashAlgo:  a.HashAlgo,
		Weight:    weight,
	}
}

// CompatibleAlgorithms returns true if the signature and hash algorithms are compatible.
func CompatibleAlgorithms(sigAlgo crypto.SigningAlgorithm, hashAlgo hash.HashingAlgorithm) bool {
	switch sigAlgo {
	case crypto.ECDSAP256, crypto.ECDSASecp256k1:
		switch hashAlgo {
		case hash.SHA2_256, hash.SHA3_256:
			return true
		}
	}
	return false
}
