package unittest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/keys"
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

func AddressFixture() flow.Address {
	return flow.RootAddress
}

func AccountSignatureFixture() flow.AccountSignature {
	return flow.AccountSignature{
		Account:   AddressFixture(),
		Signature: []byte{1, 2, 3, 4},
	}
}

func BlockFixture() flow.Block {
	return flow.Block{
		Header:                BlockHeaderFixture(),
		NewIdentities:         IdentityListFixture(3),
		GuaranteedCollections: GuaranteedCollectionsFixture(3),
	}
}

func BlockHeaderFixture() flow.Header {
	return flow.Header{
		Parent: crypto.Hash("parent"),
		Number: 100,
	}
}

func GuaranteedCollectionsFixture(n int) []*flow.GuaranteedCollection {
	ret := make([]*flow.GuaranteedCollection, n)
	for i := 0; i < n; i++ {
		ret[i] = &flow.GuaranteedCollection{
			CollectionHash: []byte(fmt.Sprintf("hash %d", i)),
			Signatures:     []crypto.Signature{[]byte(fmt.Sprintf("signature %d A", i)), []byte(fmt.Sprintf("signature %d B", i))},
		}
	}
	return ret
}

func FlowCollectionFixture(n int) flow.Collection {
	col := make([]flow.Fingerprint, 0)

	for i := 0; i < n; i++ {
		TransactionFixture(func(t *flow.Transaction) {
			t.Script = []byte(fmt.Sprintf("pub fun main() { print(\"%d\")}", i))
		})
	}

	return flow.Collection{Transactions: col}
}

func TransactionFixture(n ...func(t *flow.Transaction)) flow.Transaction {
	tx := flow.Transaction{TransactionBody: flow.TransactionBody{
		Script:             []byte("pub fun main() {}"),
		ReferenceBlockHash: flow.Fingerprint(HashFixture(32)),
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       AddressFixture(),
		ScriptAccounts:     []flow.Address{AddressFixture()},
		Signatures:         []flow.AccountSignature{AccountSignatureFixture()},
	}}
	if len(n) > 0 {
		n[0](&tx)
	}
	return tx
}

func AccountFixture() flow.Account {
	return flow.Account{
		Address: AddressFixture(),
		Balance: 10,
		Code:    []byte("pub fun main() {}"),
		Keys:    []flow.AccountPublicKey{AccountPublicKeyFixture()},
	}
}

func AccountPublicKeyFixture() flow.AccountPublicKey {
	return flow.AccountPublicKey{
		PublicKey: PublicKeyFixtures()[0],
		SignAlgo:  crypto.ECDSA_P256,
		HashAlgo:  crypto.SHA3_256,
		Weight:    keys.PublicKeyWeightThreshold,
	}
}

func EventFixture(n ...func(e *flow.Event)) flow.Event {

	event := flow.Event{
		Type: "Transfer",
		// TODO: create proper fixture
		// Values: map[string]interface{}{
		// 	"to":   flow.ZeroAddress,
		// 	"from": flow.ZeroAddress,
		// 	"id":   1,
		// },
		Payload: []byte{},
	}
	if len(n) >= 1 {
		n[0](&event)
	}
	return event
}

func HashFixture(size int) crypto.Hash {
	hash := make(crypto.Hash, size)
	for i := 0; i < size; i++ {
		hash[i] = byte(i)
	}
	return hash
}

func IdentifierFixture() flow.Identifier {
	var id flow.Identifier
	_, _ = rand.Read(id[:])
	return id
}

// IdentityFixture returns a
func IdentityFixture() flow.Identity {
	return flow.Identity{
		NodeID:  IdentifierFixture(),
		Address: "address",
		Role:    flow.RoleConsensus,
		Stake:   1000,
	}
}

// IdentityListFixture returns a list of node identity objects. The identities
// can be customized (ie. set their role) by passing in a function that modifies
// the input identities as required.
func IdentityListFixture(n int, opts ...func(*flow.Identity)) flow.IdentityList {
	nodes := make(flow.IdentityList, n)

	for i := 0; i < n; i++ {
		node := IdentityFixture()
		node.Address = fmt.Sprintf("address-%d", i+1)
		for _, opt := range opts {
			opt(&node)
		}
		nodes[i] = node
	}

	return nodes
}
