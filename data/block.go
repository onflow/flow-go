package data

import (
	"time"
)

// Block represents a series of Collections (collection of Transactions) denoted with a current Status.
type Block struct {
	Number            uint64
	Timestamp         time.Time
	PrevBlockHash     Hash
	Status            BlockStatus
	CollectionHashes  []Collection
	TransactionHashes []Transaction
}
