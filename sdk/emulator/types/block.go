package types

import (
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dapperlabs/flow-go/pkg/crypto"
)

// Block is a naive data structure used to represent blocks in the emulator.
type Block struct {
	Number            uint64
	Timestamp         time.Time
	PreviousBlockHash crypto.Hash
	TransactionHashes []crypto.Hash
	Index             int
}

// Hash returns the hash of this block.
func (b *Block) Hash() crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)

	d, _ := rlp.EncodeToBytes([]interface{}{
		b.Number,
		b.PreviousBlockHash,
	})

	return hasher.ComputeHash(d)
}

// AddTransaction adds a hash to the list of transaction hashes.
func (b *Block) AddTransaction(txHash crypto.Hash) {
	b.TransactionHashes = append(b.TransactionHashes, txHash)
}

// GenesisBlock returns the genesis block for an emulated blockchain.
func GenesisBlock() *Block {
	return &Block{
		Number:            0,
		Timestamp:         time.Now(),
		PreviousBlockHash: nil,
		TransactionHashes: make([]crypto.Hash, 0),
	}
}
