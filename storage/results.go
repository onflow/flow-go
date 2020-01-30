// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Results interface {

	// Store will store an execution result.
	Store(result *flow.ExecutionResult) error

	// ByID will retrieve an execution result by its ID.
	ByID(resultID flow.Identifier) (*flow.ExecutionResult, error)
}
