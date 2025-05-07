package flow

// Collection is an ordered list of transactions.
// Collections form a part of the payload of cluster blocks, produced by Collection Nodes.
// Every Collection maps 1-1 to a Chunk, which is used for transaction execution.
type Collection struct {
	Transactions []*TransactionBody
}

// Light returns a LightCollection, which contains only the list of transaction IDs from the Collection.
func (c Collection) Light() LightCollection {
	lc := LightCollection{Transactions: make([]Identifier, 0, len(c.Transactions))}
	for _, tx := range c.Transactions {
		lc.Transactions = append(lc.Transactions, tx.ID())
	}
	return lc
}

// ID returns a cryptographic commitment to the Collection.
// The ID of a Collection is equivalent to the ID of its corresponding LightCollection.
func (c Collection) ID() Identifier {
	return c.Light().ID()
}

// Checksum returns the collection's ID.
// Deprecated: This is needed temporarily until further malleability is done, because many components assume Collection implements Entity
// TODO(malleability): remove this function
func (c Collection) Checksum() Identifier {
	return c.ID()
}

// Len returns the number of transactions in the collection.
func (c Collection) Len() int {
	return len(c.Transactions)
}

// LightCollection contains cryptographic commitments to the constituent transactions instead of transaction bodies.
// It is used for indexing transactions by collection and for computing the collection fingerprint.
type LightCollection struct {
	Transactions []Identifier
}

// ID returns a cryptographic commitment to the LightCollection.
// The ID of a LightCollection is equivalent to the ID for its corresponding Collection.
func (lc LightCollection) ID() Identifier {
	return MakeID(lc)
}

// Checksum returns the collection's ID.
// Deprecated: This is needed temporarily until further malleability is done, because many components assume Collection implements Entity
// TODO(malleability): remove this function
func (c LightCollection) Checksum() Identifier {
	return c.ID()
}

// Len returns the number of transactions in the collection.
func (lc LightCollection) Len() int {
	return len(lc.Transactions)
}
