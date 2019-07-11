package types

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type Transaction struct {
	Script    []byte
	Nonce     uint64
	Registers []TransactionRegister
	Chunks    [][]byte
}

type SignedTransaction struct {
	Transaction      Transaction
	ScriptSignatures []crypto.Signature
	PayerSignature   crypto.Signature
}
