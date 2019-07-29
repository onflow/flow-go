package mocks

import (
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/types"
)

func MockAddress() types.Address {
	return types.ZeroAddress()
}

func MockSignedTransaction() *types.SignedTransaction {
	return &types.SignedTransaction{
		Script:       []byte(""),
		Nonce:        1,
		ComputeLimit: 10,
		ComputeUsed:  0,
		Timestamp:    time.Now(),
		PayerSignature: types.AccountSignature{
			Account: MockAddress(),
		},
	}
}
