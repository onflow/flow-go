package emulator

import (
	"fmt"
	"strings"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// ErrBlockNotFound indicates that a block specified by hash or number cannot be found.
type ErrBlockNotFound struct {
	BlockHash crypto.Hash
	BlockNum  uint64
}

func (e *ErrBlockNotFound) Error() string {
	if e.BlockNum == 0 {
		return fmt.Sprintf("Block with hash %x cannot be found", e.BlockHash)
	}

	return fmt.Sprintf("Block number %d cannot be found", e.BlockNum)
}

// ErrTransactionNotFound indicates that a transaction specified by hash cannot be found.
type ErrTransactionNotFound struct {
	TxHash crypto.Hash
}

func (e *ErrTransactionNotFound) Error() string {
	return fmt.Sprintf("Transaction with hash %x cannot be found", e.TxHash)
}

// ErrAccountNotFound indicates that an account specified by address cannot be found.
type ErrAccountNotFound struct {
	Address flow.Address
}

func (e *ErrAccountNotFound) Error() string {
	return fmt.Sprintf("Account with address %s cannot be found", e.Address)
}

// ErrDuplicateTransaction indicates that a transaction has already been submitted.
type ErrDuplicateTransaction struct {
	TxHash crypto.Hash
}

func (e *ErrDuplicateTransaction) Error() string {
	return fmt.Sprintf("Transaction with hash %x has already been submitted", e.TxHash)
}

// ErrMissingSignature indicates that a transaction is missing a required signature.
type ErrMissingSignature struct {
	Account flow.Address
}

func (e *ErrMissingSignature) Error() string {
	return fmt.Sprintf("Account %s does not have sufficient signatures", e.Account)
}

// ErrInvalidSignaturePublicKey indicates that signature uses an invalid public key.
type ErrInvalidSignaturePublicKey struct {
	Account flow.Address
}

func (e *ErrInvalidSignaturePublicKey) Error() string {
	return fmt.Sprintf("Public key used for signing does not exist on account %s", e.Account)
}

// ErrInvalidSignatureAccount indicates that a signature references a nonexistent account.
type ErrInvalidSignatureAccount struct {
	Account flow.Address
}

func (e *ErrInvalidSignatureAccount) Error() string {
	return fmt.Sprintf("Account with address %s does not exist", e.Account)
}

// ErrInvalidTransaction indicates that a submitted transaction is invalid (missing required fields).
type ErrInvalidTransaction struct {
	TxHash        crypto.Hash
	MissingFields []string
}

func (e *ErrInvalidTransaction) Error() string {
	return fmt.Sprintf(
		"Transaction with hash %x is invalid (missing required fields): %s",
		e.TxHash,
		strings.Join(e.MissingFields, ", "),
	)
}

// ErrInvalidStateVersion indicates that a state version hash provided is invalid.
type ErrInvalidStateVersion struct {
	Version crypto.Hash
}

func (e *ErrInvalidStateVersion) Error() string {
	return fmt.Sprintf("World State with version hash %x is invalid", e.Version)
}

// ErrPendingBlockCommitBeforeExecution indicates that the current pending block has not been executed (cannot commit).
type ErrPendingBlockCommitBeforeExecution struct {
	BlockHash crypto.Hash
}

func (e *ErrPendingBlockCommitBeforeExecution) Error() string {
	return fmt.Sprintf("Pending block with hash %x cannot be commited before execution", e.BlockHash)
}

// ErrPendingBlockNotEmpty indicates that the current pending block contains previously added transactions.
type ErrPendingBlockNotEmpty struct {
	BlockHash crypto.Hash
}

func (e *ErrPendingBlockNotEmpty) Error() string {
	return fmt.Sprintf("Pending block with hash %x is not empty", e.BlockHash)
}

// ErrPendingBlockMidExecution indicates that the current pending block is mid-execution.
type ErrPendingBlockMidExecution struct {
	BlockHash crypto.Hash
}

func (e *ErrPendingBlockMidExecution) Error() string {
	return fmt.Sprintf("Pending block with hash %x is currently being executed", e.BlockHash)
}

// ErrPendingBlockTransactionsExhausted indicates that the current pending block has finished executing (no more transactions to execute).
type ErrPendingBlockTransactionsExhausted struct {
	BlockHash crypto.Hash
}

func (e *ErrPendingBlockTransactionsExhausted) Error() string {
	return fmt.Sprintf("Pending block with hash %x contains no more transactions to execute", e.BlockHash)
}

// ErrStorage indicates that an error occurred in the storage provider.
type ErrStorage struct {
	inner error
}

func (e *ErrStorage) Error() string {
	return fmt.Sprintf("storage failure: %v", e.inner)
}

func (e *ErrStorage) Unwrap() error {
	return e.inner
}
