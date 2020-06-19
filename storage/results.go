// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type ExecutionResults interface {

	// Store stores an execution result.
	Store(result *flow.ExecutionResult) error

	// ByID retrieves an execution result by its ID.
	ByID(resultID flow.Identifier) (*flow.ExecutionResult, error)

	// Index indexes an execution result by block ID.
	Index(blockID flow.Identifier, resultID flow.Identifier) error

	// ByBlockID retrieves an execution result by block ID.
	ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error)
}
