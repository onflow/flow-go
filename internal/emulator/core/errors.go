package core

import (
	"fmt"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// ErrDuplicateTransaction indicates that a transaction has already been submitted.
type ErrDuplicateTransaction struct {
	TxHash crypto.Hash
}

func (e *ErrDuplicateTransaction) Error() string {
	return fmt.Sprintf("Transaction with hash %s has already been submitted", e.TxHash)
}

// ErrInvalidSignaturePublicKey indicates that signature uses an invalid public key.
type ErrInvalidSignaturePublicKey struct {
	Account   types.Address
	PublicKey []byte
}

func (e *ErrInvalidSignaturePublicKey) Error() string {
	return fmt.Sprintf("Public key %s does not exist on account %s", e.PublicKey, e.Account)
}

// ErrInvalidSignatureAccount indicates that a signature references a nonexistent account.
type ErrInvalidSignatureAccount struct {
	Account types.Address
}

func (e *ErrInvalidSignatureAccount) Error() string {
	return fmt.Sprintf("Account with address %s does not exist", e.Account)
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
