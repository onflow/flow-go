package flow

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
)

// Account represents an account on the Flow network.
//
// An account can be an externally owned account or a contract account with code.
type Account struct {
	Address   Address
	Balance   uint64
	Keys      []AccountPublicKey
	Contracts map[string][]byte
}

// AccountPublicKey is a public key associated with an account.
//
// An account public key contains the public key, signing and hashing algorithms, and a key weight.
type AccountPublicKey struct {
	Index     int
	PublicKey crypto.PublicKey
	SignAlgo  crypto.SigningAlgorithm
	HashAlgo  hash.HashingAlgorithm
	SeqNumber uint64
	Weight    int
	Revoked   bool
}

func (a AccountPublicKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PublicKey []byte
		SignAlgo  crypto.SigningAlgorithm
		HashAlgo  hash.HashingAlgorithm
		SeqNumber uint64
		Weight    int
	}{
		a.PublicKey.Encode(),
		a.SignAlgo,
		a.HashAlgo,
		a.SeqNumber,
		a.Weight,
	})
}

func (a *AccountPublicKey) UnmarshalJSON(data []byte) error {
	temp := struct {
		PublicKey []byte
		SignAlgo  crypto.SigningAlgorithm
		HashAlgo  hash.HashingAlgorithm
		SeqNumber uint64
		Weight    int
	}{}
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}
	if a == nil {
		a = new(AccountPublicKey)
	}
	a.PublicKey, err = crypto.DecodePublicKey(temp.SignAlgo, temp.PublicKey)
	if err != nil {
		return err
	}
	a.SignAlgo = temp.SignAlgo
	a.HashAlgo = temp.HashAlgo
	a.SeqNumber = temp.SeqNumber
	a.Weight = temp.Weight
	return nil
}

// Validate returns an error if this account key is invalid.
//
// An account key can be invalid for the following reasons:
// - It specifies an incompatible signature/hash algorithm pairing
// - (TODO) It specifies a negative key weight
func (a AccountPublicKey) Validate() error {
	if !CompatibleAlgorithms(a.SignAlgo, a.HashAlgo) {
		return fmt.Errorf(
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

func (a AccountPrivateKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PrivateKey []byte
		SignAlgo   crypto.SigningAlgorithm
		HashAlgo   hash.HashingAlgorithm
	}{
		a.PrivateKey.Encode(),
		a.SignAlgo,
		a.HashAlgo,
	})
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
