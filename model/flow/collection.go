package flow

import "github.com/onflow/flow-go/model/fingerprint"

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

// ID returns a cryptographic commitment to the receiver Collection.
// The ID of a Collection is equivalent to the ID of its corresponding LightCollection.
func (c Collection) ID() Identifier {
	return c.Light().ID()
}

func (c Collection) Len() int {
	return len(c.Transactions)
}

func (c Collection) Checksum() Identifier {
	return c.Light().Checksum()
}

// TODO: why is this necessary?
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

// LightCollection contains cryptographic commitments to the constituent transactions instead of transaction bodies.
// It is used for indexing transactions by collection and for computing the collection fingerprint.
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
