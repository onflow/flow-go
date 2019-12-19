package flow

import (
	"bytes"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/hash"
)

// Collection is set of transactions (transaction set)
type Collection struct {
	Transactions []model.Commit
}

// Commit returns the canonical hash of this collection.
func (c *Collection) Commit() model.Commit {
	return model.Commit(hash.DefaultHasher.ComputeHash(encoding.DefaultEncoder.MustEncode(c)))
}

// AddItem adds another entityID to the collection
func (c *Collection) AddItem(item model.Entity) {
	c.Transactions = append(c.Transactions, item.Commit())
}

// GetItem returns a single item from the collection with the proof that this item is located at this index
func (c *Collection) GetItem(index uint64) (commit model.Commit, proof []byte) {
	return c.Transactions[index], nil
}

// GetItem returns a range of elements from the collection
// and provides an aggregated proof
func (c *Collection) GetItems(startIndex uint64, length uint64) (commits []model.Commit, proof []byte) {
	return c.Transactions[startIndex : startIndex+length], nil
}

// Reset resets all transactions inside the collection
func (c *Collection) Reset() {
	c.Transactions = make([]model.Commit, 0)
}

// IsEmpty returns true if the collection is empty
func (c Collection) IsEmpty() bool {
	return len(c.Transactions) == 0
}

// Size returns len of transaction slice
func (c *Collection) Size() int {
	return len(c.Transactions)
}

func (c *Collection) ID() model.Identifier {
	var id model.Identifier
	copy(id[:], hash.DefaultHasher.ComputeHash(encoding.DefaultEncoder.MustEncode(c)))
	return id
}

// Note that this is the basic version of the List, we need to substitute it with something like Merkel tree at some point
type CollectionList struct {
	collections []Collection
}

func (cl *CollectionList) Commit() model.Commit {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	for _, item := range cl.collections {
		hasher.Add(item.Commit())
	}
	return model.Commit(hasher.SumHash())
}

func (cl *CollectionList) AppendItem(ch Collection) {
	cl.collections = append(cl.collections, ch)
}

func (cl *CollectionList) Items() []Collection {
	return cl.collections
}

// ByCommit returns an entity from the list by entity commit
func (cl *CollectionList) ByCommit(c model.Commit) Collection {
	for _, item := range cl.collections {
		if bytes.Equal(item.Commit(), c) {
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
func (cl *CollectionList) ByIndexWithProof(i uint64) (Collection, model.MembershipProof) {
	return cl.collections[i], nil
}
