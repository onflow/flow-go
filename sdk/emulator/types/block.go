package types

import (
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/golang/protobuf/ptypes"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/grpc/services/observe"
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

	var prevBlockHash []byte

	if b.PreviousBlockHash != nil {
		prevBlockHash = b.PreviousBlockHash
	}

	blockMsg := &observe.Block{
		Hash:              b.Hash(),
		Number:            b.Number,
		PrevBlockHash:     prevBlockHash,
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
