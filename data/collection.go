package data

// Collection represents a "collection" of Transactions.
type Collection struct {
	Id 					uint64
	CollectionHash 		Hash
	TransactionHashes	map[Hash]Transaction
}
