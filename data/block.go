package data

import (
	"time"
)

type Block struct {
	Id					uint64
	BlockHash			Hash
	Timestamp			time.Time
	PrevBlockHash		Hash
	Status				Status
	CollectionHashes	map[Hash]Collection
	TransactionHashes	map[Hash]Transaction
}
