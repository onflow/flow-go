// Package mocks provides mocked values of common types for use in unit tests.
package mocks

import (
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/types"
)

func MockAddress() types.Address {
	return types.ZeroAddress()
}

func MockAccountSignature() types.AccountSignature {
	return types.AccountSignature{
		Account:   MockAddress(),
		Signature: []byte{},
	}
}

func MockSignedTransaction() types.SignedTransaction {
	return types.SignedTransaction{
		Script:         []byte("fun main() {}"),
		Nonce:          1,
		ComputeLimit:   10,
		ComputeUsed:    0,
		Timestamp:      time.Now().In(time.UTC),
		PayerSignature: MockAccountSignature(),
	}
}
