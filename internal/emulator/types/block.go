package types

import (
	"time"

	"github.com/dapperlabs/bamboo-node/internal/emulator/utils"
	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

type Block struct {
	Height            uint64
	Timestamp         time.Time
	PreviousBlockHash crypto.Hash
	TransactionHashes []crypto.Hash
}

func (b *Block) Hash() crypto.Hash {
	bytes := utils.EncodeAsBytes(
		b.Height,
		b.Timestamp,
		b.PreviousBlockHash,
		b.TransactionHashes,
	)
	return crypto.NewHash(bytes)
}

func GenesisBlock() *Block {
	return &Block{
		Height:            0,
		Timestamp:         time.Now(),
		PreviousBlockHash: crypto.Hash{},
		TransactionHashes: make([]crypto.Hash, 0),
	}
}
