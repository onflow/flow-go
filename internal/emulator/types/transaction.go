package types

import (
	"time"

	"github.com/dapperlabs/bamboo-node/internal/emulator/utils"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type RawTransaction struct {
	Timestamp	time.Time
	Nonce        uint64
	Script       []byte
	ComputeLimit uint64
}

func (tx *RawTransaction) Hash() crypto.Hash {
	bytes := utils.EncodeAsBytes(
		tx.Timestamp,
		tx.Nonce,
		tx.Script,
		tx.ComputeLimit,
	)
	return crypto.NewHash(bytes)
}

type SignedTransaction struct {
	Transaction    *RawTransaction
	PayerSignature crypto.Signature
}

func (tx *SignedTransaction) Hash() crypto.Hash {
	return tx.Transaction.Hash()
}
