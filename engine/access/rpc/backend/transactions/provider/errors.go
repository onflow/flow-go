package provider

import "errors"

var (
	// ErrNotASystemTransaction is returned when a transaction is requested that is not a system transaction.
	ErrNotASystemTransaction = errors.New("not a system transaction")

	// ErrBlockCollectionNotFound is returned when a collection within the block is not found.
	ErrBlockCollectionNotFound = errors.New("block collection not found")
)
