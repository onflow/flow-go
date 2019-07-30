package unittest

import (
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/types"
)

func AddressFixture() types.Address {
	return types.ZeroAddress()
}

func AccountSignatureFixture() types.AccountSignature {
	return types.AccountSignature{
		Account:   MockAddress(),
		Signature: []byte{},
	}
}

func SignedTransactionFixture() types.SignedTransaction {
	return types.SignedTransaction{
		Script:         []byte("fun main() {}"),
		Nonce:          1,
		ComputeLimit:   10,
		ComputeUsed:    0,
		Timestamp:      time.Now().In(time.UTC),
		PayerSignature: MockAccountSignature(),
	}
}
