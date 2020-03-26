package flow

// Collection is set of transactions.
type Collection struct {
	Transactions []*TransactionBody
}

// CollectionFromTransactions creates a new collection from the list of
// transactions.
func CollectionFromTransactions(transactions []*Transaction) Collection {
	coll := Collection{Transactions: make([]*TransactionBody, 0, len(transactions))}
	for _, tx := range transactions {
		coll.Transactions = append(coll.Transactions, &tx.TransactionBody)
	}
	return coll
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
	return CollectionGuarantee{
		CollectionID: c.ID(),
	}
}

func (c Collection) ID() Identifier {
	return c.Light().ID()
}

func (c Collection) Checksum() Identifier {
	return c.Light().Checksum()
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

//  ByIndexWithProof returns an entity from the list by index and proof of membership
func (cl *CollectionList) ByIndexWithProof(i uint64) (*Collection, Proof) {
	return cl.collections[i], nil
}
