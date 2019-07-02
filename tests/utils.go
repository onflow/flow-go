package tests

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
)

// MockTransaction generates a dummy transaction to be used for testing.
func MockTransaction(nonce uint64) *data.Transaction {
	return &data.Transaction{
		ToAddress:      crypto.BytesToAddress([]byte{}),
		Script:         []byte{},
		Nonce:          nonce,
		ComputeLimit:   10,
		ComputeUsed:    0,
		PayerSignature: []byte{},
		Status:         data.TxPending,
	}
}
