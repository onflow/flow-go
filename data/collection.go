package data

// Collection represents a "collection" of Transactions.
type Collection struct {
	TransactionHashes []Hash
}

// Hash computes the hash over the necessary Collection data.
func (c Collection) Hash() Hash {
	bytes := EncodeAsBytes(c.TransactionHashes)
	return NewHash(bytes)
}