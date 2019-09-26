package unittest

import (
	"github.com/dapperlabs/flow-go/pkg/types"
)

func AddressFixture() types.Address {
	return types.ZeroAddress()
}

func AccountSignatureFixture() types.AccountSignature {
	return types.AccountSignature{
		Account:   AddressFixture(),
		Signature: []byte{},
	}
}

func TransactionFixture() types.Transaction {
	return types.Transaction{
		Script:         []byte("fun main() {}"),
		Nonce:          1,
		ComputeLimit:   10,
		PayerSignature: AccountSignatureFixture(),
	}
}
