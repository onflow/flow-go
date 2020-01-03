package flow

import (
	"bytes"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/hash"
)

// Collection is set of transactions (transaction set)
type Collection struct {
	Transactions []Fingerprint
}

// Fingerprint returns the canonical hash of this collection.
func (c *Collection) Fingerprint() Fingerprint {
	return Fingerprint(hash.DefaultHasher.ComputeHash(encoding.DefaultEncoder.MustEncode(c)))
}

// AddItem adds another entityID to the collection
func (c *Collection) Add(item Entity) {
	c.Transactions = append(c.Transactions, item.Fingerprint())
}

// GetItem returns a single item from the collection with the proof that this item is located at this index
func (c *Collection) GetItem(index uint64) (fingerprint Fingerprint, proof []byte) {
	return c.Transactions[index], nil
}

// GetItem returns a range of elements from the collection
// and provides an aggregated proof
func (c *Collection) GetItems(startIndex uint64, length uint64) (fingerprints []Fingerprint, proof []byte) {
	return c.Transactions[startIndex : startIndex+length], nil
}

// Reset resets all transactions inside the collection
func (c *Collection) Reset() {
	c.Transactions = make([]Fingerprint, 0)
}

// IsEmpty returns true if the collection is empty
func (c Collection) IsEmpty() bool {
	return len(c.Transactions) == 0
}

// Size returns len of transaction slice
func (c *Collection) Size() int {
	return len(c.Transactions)
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
