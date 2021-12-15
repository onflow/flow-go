package flow

import (
	"encoding/json"
	"errors"

	"github.com/ipfs/go-cid"
)

var ErrNoChunks = errors.New("execution result has no chunks")

// ExecutionResult is cryptographic commitment to the computation
// result(s) from executing a block
type ExecutionResult struct {
	PreviousResultID Identifier // commit of the previous ER
	BlockID          Identifier // commit of the current block
	Chunks           ChunkList
	ServiceEvents    ServiceEventList
	ExecutionDataCID cid.Cid
}

// ID returns the hash of the execution result body
func (er ExecutionResult) ID() Identifier {
	return MakeID(er)
}

// Checksum ...
func (er ExecutionResult) Checksum() Identifier {
	return MakeID(er)
}

// ValidateChunksLength checks whether the number of chuncks is zero.
//
// It returns false if the number of chunks is zero (invalid).
// By protocol definition, each ExecutionReceipt must contain at least one
// chunk (system chunk).
func (er ExecutionResult) ValidateChunksLength() bool {
	return er.Chunks.Len() != 0
}

// FinalStateCommitment returns the Execution Result's commitment to the final
// execution state of the block, i.e. the last chunk's output state.
// Error returns:
//  * ErrNoChunks: if there are no chunks (ExecutionResult is malformed)
func (er ExecutionResult) FinalStateCommitment() (StateCommitment, error) {
	if !er.ValidateChunksLength() {
		return DummyStateCommitment, ErrNoChunks
	}
	return er.Chunks[er.Chunks.Len()-1].EndState, nil
}

// InitialStateCommit returns a commitment to the execution state used as input
// for computing the block the block, i.e. the leading chunk's input state.
// Error returns:
//  * ErrNoChunks: if there are no chunks (ExecutionResult is malformed)
func (er ExecutionResult) InitialStateCommit() (StateCommitment, error) {
	if !er.ValidateChunksLength() {
		return DummyStateCommitment, ErrNoChunks
	}
	return er.Chunks[0].StartState, nil
}

func (er ExecutionResult) MarshalJSON() ([]byte, error) {
	type Alias ExecutionResult
	return json.Marshal(struct {
		Alias
		ID string
	}{
		Alias: Alias(er),
		ID:    er.ID().String(),
	})
}

/*******************************************************************************
GROUPING allows to split a list of results by some property
*******************************************************************************/

// ExecutionResultList is a slice of ExecutionResults with the additional
// functionality to group them by various properties
type ExecutionResultList []*ExecutionResult

// ExecutionResultGroupedList is a partition of an ExecutionResultList
type ExecutionResultGroupedList map[Identifier]ExecutionResultList

// ExecutionResultGroupingFunction is a function that assigns an identifier to each ExecutionResult
type ExecutionResultGroupingFunction func(*ExecutionResult) Identifier

// GroupBy partitions the ExecutionResultList. All ExecutionResults that are
// mapped by the grouping function to the same identifier are placed in the same group.
// Within each group, the order and multiplicity of the ExecutionResults is preserved.
func (l ExecutionResultList) GroupBy(grouper ExecutionResultGroupingFunction) ExecutionResultGroupedList {
	groups := make(map[Identifier]ExecutionResultList)
	for _, r := range l {
		groupID := grouper(r)
		groups[groupID] = append(groups[groupID], r)
	}
	return groups
}

// GroupByPreviousResultID partitions the ExecutionResultList by the their PreviousResultIDs.
// Within each group, the order and multiplicity of the ExecutionResults is preserved.
func (l ExecutionResultList) GroupByPreviousResultID() ExecutionResultGroupedList {
	grouper := func(r *ExecutionResult) Identifier { return r.PreviousResultID }
	return l.GroupBy(grouper)
}

// GroupByExecutedBlockID partitions the ExecutionResultList by the IDs of the executed blocks.
// Within each group, the order and multiplicity of the ExecutionResults is preserved.
func (l ExecutionResultList) GroupByExecutedBlockID() ExecutionResultGroupedList {
	grouper := func(r *ExecutionResult) Identifier { return r.BlockID }
	return l.GroupBy(grouper)
}

// Size returns the number of ExecutionResults in the list
func (l ExecutionResultList) Size() int {
	return len(l)
}

// GetGroup returns the ExecutionResults that were mapped to the same identifier by the
// grouping function. Returns an empty (nil) ExecutionResultList if groupID does not exist.
func (g ExecutionResultGroupedList) GetGroup(groupID Identifier) ExecutionResultList {
	return g[groupID]
}

// NumberGroups returns the number of groups
func (g ExecutionResultGroupedList) NumberGroups() int {
	return len(g)
}

// Lookup generates a map from ExecutionResult ID to ExecutionResult
func (l ExecutionResultList) Lookup() map[Identifier]*ExecutionResult {
	resultsByID := make(map[Identifier]*ExecutionResult, len(l))
	for _, result := range l {
		resultsByID[result.ID()] = result
	}
	return resultsByID
}
