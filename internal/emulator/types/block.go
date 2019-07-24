package types

import (
	"time"

	"github.com/golang/protobuf/ptypes"

	"github.com/dapperlabs/bamboo-node/grpc/services/observe"
	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

type Block struct {
	Number            uint64
	Timestamp         time.Time
	PreviousBlockHash crypto.Hash
	TransactionHashes []crypto.Hash
}

func (b *Block) Hash() crypto.Hash {
	bytes := crypto.EncodeAsBytes(
		b.Number,
		b.Timestamp,
		b.PreviousBlockHash,
		b.TransactionHashes,
	)
	return crypto.NewHash(bytes)
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
		PreviousBlockHash: crypto.Hash{},
		TransactionHashes: make([]crypto.Hash, 0),
	}
}
