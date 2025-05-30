package flow

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/fingerprint"
)

// Collection is set of transactions.
//
//structwrite:immutable - mutations allowed only within the constructor
type Collection struct {
	Transactions []*TransactionBody
}

// NewCollection creates a new instance of Collection.
// Construction Collection allowed only within the constructor
func NewCollection(transactions []*TransactionBody) *Collection {
	return &Collection{Transactions: transactions}
}

// CollectionFromTransactions creates a new collection from the list of
// transactions.
func CollectionFromTransactions(transactions []*Transaction) Collection {
	txs := make([]*TransactionBody, 0, len(transactions))

	for _, tx := range transactions {
		txs = append(txs, &tx.TransactionBody)
	}
	return *NewCollection(txs)
}

// Light returns the light, reference-only version of the collection.
func (c Collection) Light() LightCollection {
	lc := LightCollection{Transactions: make([]Identifier, 0, len(c.Transactions))}
	for _, tx := range c.Transactions {
		lc.Transactions = append(lc.Transactions, tx.ID())
	}
	return lc
}

// Guarantee returns a collection guarantee for this collection.
func (c *Collection) Guarantee() CollectionGuarantee {
	return *NewCollectionGuarantee(
		c.ID(),
		Identifier{},
		ChainID(""),
		nil,
		crypto.Signature{},
	)
}

func (c Collection) ID() Identifier {
	return c.Light().ID()
}

func (c Collection) Len() int {
	return len(c.Transactions)
}

func (c Collection) Checksum() Identifier {
	return c.Light().Checksum()
}

func (c Collection) Fingerprint() []byte {
	var txs []byte
	for _, tx := range c.Transactions {
		txs = append(txs, tx.Fingerprint()...)
	}

	return fingerprint.Fingerprint(struct {
		Transactions []byte
	}{
		Transactions: txs,
	})
}

// LightCollection is a collection containing references to the constituent
// transactions rather than full transaction bodies. It is used for indexing
// transactions by collection and for computing the collection fingerprint.
type LightCollection struct {
	Transactions []Identifier
}

func (lc LightCollection) ID() Identifier {
	return MakeID(lc)
}

func (lc LightCollection) Checksum() Identifier {
	return MakeID(lc)
}

func (lc LightCollection) Len() int {
	return len(lc.Transactions)
}

func (lc LightCollection) Has(txID Identifier) bool {
	for _, id := range lc.Transactions {
		if txID == id {
			return true
		}
	}
	return false
}

// Note that this is the basic version of the List, we need to substitute it with something like Merkle tree at some point
type CollectionList struct {
	collections []*Collection
}

func (cl *CollectionList) Fingerprint() Identifier {
	return MerkleRoot(GetIDs(cl.collections)...)
}

func (cl *CollectionList) Insert(ch *Collection) {
	cl.collections = append(cl.collections, ch)
}

func (cl *CollectionList) Items() []*Collection {
	return cl.collections
}

// ByChecksum returns an entity from the list by entity fingerprint
func (cl *CollectionList) ByChecksum(cs Identifier) (*Collection, bool) {
	for _, coll := range cl.collections {
		if coll.Checksum() == cs {
			return coll, true
		}
	}
	return nil, false
}

// ByIndex returns an entity from the list by index
func (cl *CollectionList) ByIndex(i uint64) *Collection {
	return cl.collections[i]
}
