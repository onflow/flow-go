package access

import (
	"fmt"

	"github.com/dapperlabs/bamboo-emulator/data"
)

// BlockNotFoundError indicates that a block could not be found.
type BlockNotFoundError struct {
	blockHash   *data.Hash
	blockNumber uint64
}

func (e *BlockNotFoundError) Error() string {
	if e.blockHash != nil {
		return fmt.Sprintf("Block with hash %v does not exist", e.blockHash)
	}

	if e.blockNumber != 0 {
		return fmt.Sprintf("Block with number %d does not exist", e.blockNumber)
	}

	return "Block does not exist"
}

// TransactionNotFoundError indicates that a transaction could not be found.
type TransactionNotFoundError struct {
	txHash data.Hash
}

func (e *TransactionNotFoundError) Error() string {
	return fmt.Sprintf("Transaction with hash %s does not exist", e.txHash)
}

// DuplicateTransactionError indicates that a transaction has already been submitted.
type DuplicateTransactionError struct {
	txHash data.Hash
}

func (e *DuplicateTransactionError) Error() string {
	return fmt.Sprintf("Transaction with hash %s has already been submitted", e.txHash)
}
