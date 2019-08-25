package types

import (
	"github.com/ethereum/go-ethereum/rlp"
)

// AccountSignature is a compound type that includes a signature, a public key
// and an account address.
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
