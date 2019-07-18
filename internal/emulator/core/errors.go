package core

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

// ErrDuplicateTransaction indicates that a transaction has already been submitted.
type ErrDuplicateTransaction struct {
	TxHash crypto.Hash
}

func (e *ErrDuplicateTransaction) Error() string {
	return fmt.Sprintf("Transaction with hash %s has already been submitted", e.TxHash)
}

// ErrInvalidTransactionSignature indicates that a transaction has an invalid signature.
type ErrInvalidTransactionSignature struct {
	TxHash crypto.Hash
}

func (e *ErrInvalidTransactionSignature) Error() string {
	return fmt.Sprintf("Transaction with hash %s has an invalid signature", e.TxHash)
}

// ErrTransactionReverted indicates that a transaction reverted.
type ErrTransactionReverted struct {
	TxHash crypto.Hash
	Err    error
}

func (e *ErrTransactionReverted) Error() string {
	return fmt.Sprintf(
		"Transaction with hash %s reverted during execution: %s",
		e.TxHash,
		e.Err.Error(),
	)
}
