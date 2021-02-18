// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type ExecutionReceipts interface {

	// Store stores an execution receipt.
	Store(result *flow.ExecutionReceipt) error

	// ByID retrieves an execution receipt by its ID.
	ByID(resultID flow.Identifier) (*flow.ExecutionReceipt, error)

	// Index indexes an execution receipt by block ID.
	Index(blockID flow.Identifier, resultID flow.Identifier) error

	// IndexByExecutor indexes an execution receipt by block ID and execution ID
	IndexByExecutor(receipt *flow.ExecutionReceipt) error

	// ByBlockID retrieves an execution receipt by block ID.
	ByBlockID(blockID flow.Identifier) (*flow.ExecutionReceipt, error)

	// ByBlockIDAllExecutionReceipts retrieves the largest list of execution receipts for the given block ID
	// which have the same execution result
	ByBlockIDAllExecutionReceipts(blockID flow.Identifier) ([]*flow.ExecutionReceipt, error)
}
