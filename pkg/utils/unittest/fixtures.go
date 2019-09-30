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
		Script:             []byte("fun main() {}"),
		ReferenceBlockHash: nil,
		Nonce:              0,
		ComputeLimit:       10,
		PayerAccount:       AddressFixture(),
		ScriptAccounts:     []types.Address{AddressFixture()},
		Signatures:         []types.AccountSignature{AccountSignatureFixture()},
	}
}
