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
	CollectionHashes  []Hash
	TransactionHashes []Hash
}

// Hash computes the hash over the necessary Block data.
func (b Block) Hash() Hash {
	bytes := EncodeAsBytes(
		b.Number,
		b.Timestamp,
		b.PrevBlockHash,
		b.CollectionHashes,
		b.TransactionHashes,
	)
	return NewHash(bytes)
}