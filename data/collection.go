package data

type Collection struct {
	Id 					uint64
	CollectionHash 		Hash
	TransactionHashes	map[Hash]Transaction
}
