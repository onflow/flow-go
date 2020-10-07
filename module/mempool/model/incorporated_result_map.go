package model

import "github.com/onflow/flow-go/model/flow"

// IncorporatedResultMap is an internal data structure for the incorporated
// results mempool. IncorporatedResults are indexed by ExecutionResult ID and
// IncorporatedBlockID
type IncorporatedResultMap struct {
	ExecutionResult     *flow.ExecutionResult
	IncorporatedResults map[flow.Identifier]*flow.IncorporatedResult // [incorporated block ID] => IncorporatedResult
}

// ID implements flow.Entity.ID for IncorporatedResultMap to make it capable of
// being stored directly in mempools and storage.
func (a IncorporatedResultMap) ID() flow.Identifier {
	return a.ExecutionResult.ID()
}

// CheckSum implements flow.Entity.CheckSum for IncorporatedResultMap to make it
// capable of being stored directly in mempools and storage. It makes the id of
// the entire IncorporatedResultMap.
func (a IncorporatedResultMap) Checksum() flow.Identifier {
	return flow.MakeID(a)
}
