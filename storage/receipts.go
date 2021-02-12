// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type ExecutionReceipts interface {

	// Store stores an execution receipt
	// and indexes the receipt by its respective Block ID
	Store(receipt *flow.ExecutionReceipt) error

	// ByID retrieves an execution receipt by its ID.
	ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error)

	// ByBlockID retrieves all known receipts for the given block ID.
	ByBlockID(blockID flow.Identifier) ([]*flow.ExecutionReceipt, error)
}
