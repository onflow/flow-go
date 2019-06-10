package data

import (
	"time"
)
// Block represents a series of Collections (collection of Transactions) denoted with a current Status.
type Block struct {
	Id					uint64
	BlockHash			Hash
	Timestamp			time.Time
	PrevBlockHash		Hash
	Status				Status
	CollectionHashes	map[Hash]Collection
	TransactionHashes	map[Hash]Transaction
}
