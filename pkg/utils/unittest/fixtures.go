package unittest

import (
	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/crypto"
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

func BlockHeaderFixture() types.BlockHeader {
	return types.BlockHeader{
		Hash:              crypto.Hash("abc"),
		PreviousBlockHash: crypto.Hash("def"),
		Number:            100,
		TransactionCount:  2000,
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

func AccountFixture() types.Account {
	return types.Account{
		Address: AddressFixture(),
		Balance: 10,
		Code:    []byte("fun main() {}"),
		Keys:    []types.AccountKey{AccountKeyFixture()},
	}
}

func AccountKeyFixture() types.AccountKey {
	return types.AccountKey{
		PublicKey: []byte{1, 2, 3},
		Weight:    constants.AccountKeyWeightThreshold,
	}
}
