package types

import (
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type Block struct {
	Height            uint64
	Timestamp         time.Time
	PreviousBlockHash crypto.Hash
	TransactionHashes []crypto.Hash
}

func (b *Block) Hash() {
	bytes := utils.EncodeAsBytes(
		b.Height,
		b.Timestamp,
		b.PreviousBlockHash,
		b.TransactionHashes,
	)
	return crypto.NewHash(bytes)
}
