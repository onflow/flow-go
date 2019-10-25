package unittest

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/constants"
)

func AddressFixture() flow.Address {
	return flow.ZeroAddress
}

func AccountSignatureFixture() flow.AccountSignature {
	return flow.AccountSignature{
		Account:   AddressFixture(),
		Signature: []byte{},
	}
}

func BlockHeaderFixture() flow.BlockHeader {
	return flow.BlockHeader{
		Hash:              crypto.Hash("abc"),
		PreviousBlockHash: crypto.Hash("def"),
		Number:            100,
		TransactionCount:  2000,
	}
}

func TransactionFixture() flow.Transaction {
	return flow.Transaction{
		Script:             []byte("fun main() {}"),
		ReferenceBlockHash: nil,
		Nonce:              0,
		ComputeLimit:       10,
		PayerAccount:       AddressFixture(),
		ScriptAccounts:     []flow.Address{AddressFixture()},
		Signatures:         []flow.AccountSignature{AccountSignatureFixture()},
	}
}

func AccountFixture() flow.Account {
	return flow.Account{
		Address: AddressFixture(),
		Balance: 10,
		Code:    []byte("fun main() {}"),
		Keys:    []flow.AccountKey{AccountKeyFixture()},
	}
}

func AccountKeyFixture() flow.AccountKey {
	return flow.AccountKey{
		PublicKey: []byte{1, 2, 3},
		Weight:    constants.AccountKeyWeightThreshold,
	}
}
