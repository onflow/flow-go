package flow

import (
	"bytes"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/hash"
)

// Collection is set of transactions.
type Collection struct {
	Transactions []TransactionBody
}

// LightCollection is a collection containing references to the constituent
// transactions rather than full transaction bodies. It is used for indexing
// transactions by collection and for computing the collection fingerprint.
type LightCollection struct {
	Transactions []Fingerprint
}

func (lc *LightCollection) Fingerprint() Fingerprint {
	return Fingerprint(hash.DefaultHasher.ComputeHash(encoding.DefaultEncoder.MustEncode(lc)))
}

// Light returns the light, reference-only version of the collection.
func (c Collection) Light() *LightCollection {
	lc := LightCollection{Transactions: make([]Fingerprint, len(c.Transactions))}

	for i := 0; i < len(c.Transactions); i++ {
		lc.Transactions[i] = c.Transactions[i].Fingerprint()
	}

	return &lc
}

// Fingerprint returns the canonical hash of this collection.
func (c *Collection) Fingerprint() Fingerprint {
	return c.Light().Fingerprint()
}

func (c *Collection) ID() Identifier {
	var id Identifier
	copy(id[:], hash.DefaultHasher.ComputeHash(encoding.DefaultEncoder.MustEncode(c)))
	return id
}

// Note that this is the basic version of the List, we need to substitute it with something like Merkle tree at some point
type CollectionList struct {
	collections []Collection
}

func (cl *CollectionList) Fingerprint() Fingerprint {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	for _, item := range cl.collections {
		hasher.Add(item.Fingerprint())
	}
	return Fingerprint(hasher.SumHash())
}

func (cl *CollectionList) Append(ch Collection) {
	cl.collections = append(cl.collections, ch)
}

func (cl *CollectionList) Items() []Collection {
	return cl.collections
}

// ByFingerprint returns an entity from the list by entity fingerprint
func (cl *CollectionList) ByFingerprint(c Fingerprint) Collection {
	for _, item := range cl.collections {
		if bytes.Equal(item.Fingerprint(), c) {
			return item
		}
	}
	return Collection{}
}

// ByIndex returns an entity from the list by index
func (cl *CollectionList) ByIndex(i uint64) Collection {
	return cl.collections[i]
}

//  ByIndexWithProof returns an entity from the list by index and proof of membership
func (cl *CollectionList) ByIndexWithProof(i uint64) (Collection, MembershipProof) {
	return cl.collections[i], nil
}
