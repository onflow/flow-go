// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type ExecutionResults interface {

	// Store stores an execution result.
	Store(result *flow.ExecutionResult) error

	// BatchStore stores an execution result in a given batch
	BatchStore(result *flow.ExecutionResult, batch BatchStorage) error

	// ByID retrieves an execution result by its ID.
	ByID(resultID flow.Identifier) (*flow.ExecutionResult, error)

	// Index indexes an execution result by block ID.
	Index(blockID flow.Identifier, resultID flow.Identifier) error

	// ForceIndex indexes an execution result by block ID overwriting existing database entry
	ForceIndex(blockID flow.Identifier, resultID flow.Identifier) error

	// BatchIndex indexes an execution result by block ID in a given batch
	BatchIndex(blockID flow.Identifier, resultID flow.Identifier, batch BatchStorage) error

	// ByBlockID retrieves an execution result by block ID.
	ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error)
}
