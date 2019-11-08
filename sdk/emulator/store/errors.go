package store

// NotFoundError is an error returned when a resource cannot be found.
type NotFoundError struct{}

func (e NotFoundError) Error() string {
	return "emulator/store: could not find resource"
}

// InvalidBlockNumberError is an error returned when a block with an invalid number
// is attempted to be inserted.
type InvalidBlockNumberError struct {
	// The height of the latest block
	currChainHeight uint64
	// The number of the block that was attempted to be inserted
	insertBlockNumber uint64
}

func NewInvalidBlockNumberError(chainHeight, blockNumber uint64) InvalidBlockNumberError {
	return InvalidBlockNumberError{
		currChainHeight:   chainHeight,
		insertBlockNumber: blockNumber,
	}
}

func (e InvalidBlockNumberError) Error() string {
	return "emulator/store: invalid block number, cannot insert block (%d) with chain height (%d)"
}
