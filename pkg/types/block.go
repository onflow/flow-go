package types

import (
	"time"

	"github.com/dapperlabs/flow-go/pkg/crypto"
)

type BlockHeader struct {
	Hash              crypto.Hash
	PreviousBlockHash crypto.Hash
	Height            uint64
	TransactionCount  uint32
}

type Block struct {
	ChainID                string
	Height                 uint64
	PreviousBlockHash      crypto.Hash
	Timestamp              time.Time
	SignedCollectionHashes []SignedCollectionHash
	BlockSeals             []BlockSeal
	Signatures             []crypto.Signature
}

type SignedCollectionHash struct {
	CollectionHash crypto.Hash
	Signatures     []crypto.Signature
}

type BlockSeal struct {
	BlockHash                  crypto.Hash
	ExecutionReceiptHash       crypto.Hash
	ExecutionReceiptSignatures []crypto.Signature
	ResultApprovalSignatures   []crypto.Signature
}
