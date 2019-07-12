package types

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type Collection struct {
	Transactions        []SignedTransaction
	FoundationBlockHash crypto.Hash
}

type SignedCollectionHash struct {
	CollectionHash crypto.Hash
	Signatures     []crypto.Signature
}
