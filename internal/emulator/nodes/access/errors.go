package access

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

// BlockNotFoundByHashError indicates that a block could not be found.
type BlockNotFoundByHashError struct {
	blockHash crypto.Hash
}

func (e *BlockNotFoundByHashError) Error() string {
	return fmt.Sprintf("Block with hash %v does not exist", e.blockHash)
}

// BlockNotFoundByNumberError indicates that a block could not be found.
type BlockNotFoundByNumberError struct {
	blockNumber uint64
}

func (e *BlockNotFoundByNumberError) Error() string {
	return fmt.Sprintf("Block with number %d does not exist", e.blockNumber)
}

// TransactionNotFoundError indicates that a transaction could not be found.
type TransactionNotFoundError struct {
	txHash crypto.Hash
}

func (e *TransactionNotFoundError) Error() string {
	return fmt.Sprintf("Transaction with hash %s does not exist", e.txHash)
}

// DuplicateTransactionError indicates that a transaction has already been submitted.
type DuplicateTransactionError struct {
	txHash crypto.Hash
}

func (e *DuplicateTransactionError) Error() string {
	return fmt.Sprintf("Transaction with hash %s has already been submitted", e.txHash)
}

// AccountNotFoundError indicates that an account could not be found.
type AccountNotFoundError struct {
	accountAddress crypto.Address
}

func (e *AccountNotFoundError) Error() string {
	return fmt.Sprintf("Account with address %s does not exist", e.accountAddress)
}

// TransactionQueueFullError indicates that the pending transaction queue is full.
type TransactionQueueFullError struct {
	txHash crypto.Hash
}

func (e *TransactionQueueFullError) Error() string {
	return fmt.Sprintf("Transaction with hash %s could not be submitted; pending queue is full", e.txHash)
}
