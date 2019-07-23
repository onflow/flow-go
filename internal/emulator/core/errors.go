package core

import (
	"fmt"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
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

// ErrInvalidStateVersion indicates that a state version hash provided is invalid.
type ErrInvalidStateVersion struct {
	Version crypto.Hash
}

func (e *ErrInvalidStateVersion) Error() string {
	return fmt.Sprintf("World State with version hash %s is invalid", e.Version)
}
