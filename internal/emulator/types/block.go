package types

import (
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/golang/protobuf/ptypes"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"
)

type Block struct {
	Number            uint64
	Timestamp         time.Time
	PreviousBlockHash crypto.Hash
	TransactionHashes []crypto.Hash
}

func (b *Block) Hash() crypto.Hash {
	// TODO: generate proper hash
	hasher, _ := crypto.NewHashAlgo(crypto.SHA3_256)

	d, _ := rlp.EncodeToBytes([]interface{}{
		b.Number,
	})

	return hasher.ComputeBytesHash(d)
}

func (b *Block) ToMessage() *observe.Block {
	timestamp, _ := ptypes.TimestampProto(b.Timestamp)

	blockMsg := &observe.Block{
		Hash:              b.Hash().Bytes(),
		Number:            b.Number,
		PrevBlockHash:     b.PreviousBlockHash.Bytes(),
		Timestamp:         timestamp,
		TransactionHashes: crypto.HashesToBytes(b.TransactionHashes),
	}

	return blockMsg
}

func GenesisBlock() *Block {
	return &Block{
		Number:            0,
		Timestamp:         time.Now(),
		PreviousBlockHash: nil,
		TransactionHashes: make([]crypto.Hash, 0),
	}
}
