package types

import (
	"time"

	"github.com/dapperlabs/bamboo-node/internal/emulator/utils"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type TxStatus int

const (
	TxPending TxStatus = iota
	TxFinalized
	TxReverted
	TxSealed
)

func (s TxStatus) String() string {
	return [...]string{"PENDING", "FINALIZED", "REVERTED", "SEALED"}[s]
}

type RawTransaction struct {
	Timestamp    time.Time
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
	Status         TxStatus
}

func (tx *SignedTransaction) Hash() crypto.Hash {
	return tx.Transaction.Hash()
}
