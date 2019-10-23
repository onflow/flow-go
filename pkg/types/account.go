package types

import (
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dapperlabs/flow-go/pkg/crypto"
)

// Account represents an account on the Flow network.
//
// An account can be an externally owned account or a contract account with code.
type Account struct {
	Address Address
	Balance uint64
	Code    []byte
	Keys    []AccountKey
}

// AccountKey is a public key associated with an account.
//
// An account key contains the public key, signing and hashing algorithms, and a key weight.
type AccountKey struct {
	PublicKey crypto.PublicKey
	SignAlgo  crypto.SigningAlgorithm
	HashAlgo  crypto.HashingAlgorithm
	Weight    int
}

// AccountSignature is a signature associated with an account.
type AccountSignature struct {
	Account   Address
	Signature []byte
}

func (a AccountSignature) Encode() []byte {
	b, _ := rlp.EncodeToBytes([]interface{}{
		a.Account.Bytes(),
		a.Signature,
	})
	return b
}

// CompatibleAlgorithms returns true if the given signing and hashing algorithms are compatible.
func CompatibleAlgorithms(signAlgo crypto.SigningAlgorithm, hashAlgo crypto.HashingAlgorithm) bool {
	t := map[crypto.SigningAlgorithm]map[crypto.HashingAlgorithm]bool{
		crypto.ECDSA_P256: {
			crypto.SHA2_256: true,
			crypto.SHA3_256: true,
		},
		crypto.ECDSA_SECp256k1: {
			crypto.SHA2_256: true,
			crypto.SHA3_256: true,
		},
	}

	return t[signAlgo][hashAlgo]
}
