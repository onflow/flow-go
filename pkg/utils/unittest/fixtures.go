package unittest

import (
	"encoding/hex"
	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
)

const PublicKeyFixtureCount = 2

func PublicKeyFixtures() [PublicKeyFixtureCount]crypto.PublicKey {
	encodedKeys := [PublicKeyFixtureCount]string{
		"3059301306072a8648ce3d020106082a8648ce3d0301070342000472b074a452d0a764a1da34318f44cb16740df1cfab1e6b50e5e4145dc06e5d151c9c25244f123e53c9b6fe237504a37e7779900aad53ca26e3b57c5c3d7030c4",
		"3059301306072a8648ce3d020106082a8648ce3d03010703420004d4423e4ca70ed9fb9bb9ce771e9393e0c3a1b66f3019ed89ab410cdf8f73d5a8ca06cc093766c1a46069cf83fce2a294d3322d55bb86ac9cb5aa805c7dd8d715",
	}

	keys := [PublicKeyFixtureCount]crypto.PublicKey{}

	for i, hexKey := range encodedKeys {
		bytesKey, _ := hex.DecodeString(hexKey)
		publicKey, _ := crypto.DecodePublicKey(crypto.ECDSA_P256, bytesKey)
		keys[i] = publicKey
	}

	return keys
}

func AddressFixture() types.Address {
	return types.ZeroAddress
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
		PublicKey: PublicKeyFixtures()[0],
		SignAlgo:  crypto.ECDSA_P256,
		HashAlgo:  crypto.SHA3_256,
		Weight:    constants.AccountKeyWeightThreshold,
	}
}
