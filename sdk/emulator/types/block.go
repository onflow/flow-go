package types

import (
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dapperlabs/flow-go/crypto"
)

// Block is a naive data structure used to represent blocks in the emulator.
type Block struct {
	Number            uint64
	PreviousBlockHash crypto.Hash
	TransactionHashes []crypto.Hash
}

// Hash returns the hash of this block.
func (b Block) Hash() crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)

	d, _ := rlp.EncodeToBytes([]interface{}{
		b.Number,
		b.PreviousBlockHash,
		b.TransactionHashes,
	})

	return hasher.ComputeHash(d)
}

// GenesisBlock returns the genesis block for an emulated blockchain.
func GenesisBlock() Block {
	return Block{
		Number:            0,
		PreviousBlockHash: nil,
		TransactionHashes: make([]crypto.Hash, 0),
	}
}
