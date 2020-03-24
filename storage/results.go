// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type ExecutionResults interface {

	// Store will store an execution result.
	Store(result *flow.ExecutionResult) error

	// ByID will retrieve an execution result by its ID.
	ByID(resultID flow.Identifier) (*flow.ExecutionResult, error)

	// Index result ID by a blockID
	Index(blockID flow.Identifier, resultID flow.Identifier) error

	// Lookup execution result by block ID
	Lookup(blockID flow.Identifier) (flow.Identifier, error)
}
