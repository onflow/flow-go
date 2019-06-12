package data

import (
	"time"

	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// Block represents a series of Collections (collection of Transactions) denoted with a current Status.
type Block struct {
	Number            uint64
	Timestamp         time.Time
	PrevBlockHash     crypto.Hash
	Status            BlockStatus
	CollectionHashes  []crypto.Hash
	TransactionHashes []crypto.Hash
}

// Hash computes the hash over the necessary Block data.
func (b Block) Hash() crypto.Hash {
	bytes := EncodeAsBytes(
		b.Number,
		b.Timestamp,
		b.PrevBlockHash,
		b.CollectionHashes,
		b.TransactionHashes,
	)
	return crypto.NewHash(bytes)
}

// MintGenesisBlock creates the genesis block of the blockchain.
func MintGenesisBlock() *Block {
	genesis := Block{
		Number: 0,
		Timestamp: time.Now(),
		PrevBlockHash: crypto.ZeroBlockHash(),
		Status: BlockSealed, 
		CollectionHashes: []crypto.Hash{},
		TransactionHashes: []crypto.Hash{},
	}
	return &genesis
}