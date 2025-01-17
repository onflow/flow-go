package model

import "github.com/onflow/flow-go/model/flow"

// IncorporatedResultMap is an internal data structure for the incorporated
// results mempool. IncorporatedResults are indexed by ExecutionResult ID and
// IncorporatedBlockID
type IncorporatedResultMap struct {
	ExecutionResult     *flow.ExecutionResult
	IncorporatedResults map[flow.Identifier]*flow.IncorporatedResult // [incorporated block ID] => IncorporatedResult
}
