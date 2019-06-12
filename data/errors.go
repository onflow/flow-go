package data

import (
	"fmt"

	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// ItemNotFoundError indicates that an item could not be found.
type ItemNotFoundError struct {
	hash crypto.Hash
}

func (e *ItemNotFoundError) Error() string {
	return fmt.Sprintf("Item with hash %s does not exist", e.hash)
}

// DuplicateItemError indicates that an item already exists.
type DuplicateItemError struct {
	hash crypto.Hash
}

func (e *DuplicateItemError) Error() string {
	return fmt.Sprintf("Item with hash %s already exists", e.hash)
}

// InvalidBlockNumberError indicates that a block number is invalid.
type InvalidBlockNumberError struct {
	blockNumber uint64
}

func (e *InvalidBlockNumberError) Error() string {
	return fmt.Sprintf("Block number %d exceeds the current blockchain height", e.blockNumber)
}
