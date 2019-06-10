package data

// Collection represents a "collection" of Transactions.
type Collection struct {
	CollectionHash    Hash
	TransactionHashes map[Hash]Transaction
}
