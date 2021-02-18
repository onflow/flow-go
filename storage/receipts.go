// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type ExecutionReceipts interface {

	// Store stores an execution receipt.
	Store(receipt *flow.ExecutionReceipt) error

	// ByID retrieves an execution receipt by its ID.
	ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error)

	// Index indexes an execution receipt by block ID.
	Index(blockID flow.Identifier, receipt flow.Identifier) error

	// ByBlockID retrieves an execution receipt by block ID.
	ByBlockID(blockID flow.Identifier) (*flow.ExecutionReceipt, error)

	// TODO: Index and ByBlockID assumes one block can only have one receipt,
	// this is a valid assumption for execution node, and therefore these two
	// functions can only be used by execution node.
	// IndexByExecutor and ByBlockIDAllExecutionReceipts allows a block to have
	// multiple receipts from different executors, and only stores one receipt per
	// executor. This is from non-execution-node's perspective.
	// We'd better split the above two use cases, and create two different receipts
	// module to avoid misuse and confusion.

	// IndexByExecutor indexes an execution receipt by block ID and execution ID
	IndexByExecutor(receipt *flow.ExecutionReceipt) error

	// ByBlockIDAllExecutionReceipts retrieves all execution receipts for a block ID
	ByBlockIDAllExecutionReceipts(blockID flow.Identifier) ([]*flow.ExecutionReceipt, error)
}
