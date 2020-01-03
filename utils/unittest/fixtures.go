package unittest

import (
	"crypto/rand"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

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
