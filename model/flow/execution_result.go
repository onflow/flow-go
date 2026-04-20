package flow

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrNoChunks = errors.New("execution result has no chunks")

// ExecutionResult is cryptographic commitment to the computation
// result(s) from executing a block
//
//structwrite:immutable - mutations allowed only within the constructor
type ExecutionResult struct {
	PreviousResultID Identifier // commit of the previous ER
	BlockID          Identifier // commit of the current block
	Chunks           ChunkList
	ServiceEvents    ServiceEventList
	ExecutionDataID  Identifier // hash commitment to flow.BlockExecutionDataRoot
}

// UntrustedExecutionResult is an untrusted input-only representation of an ExecutionResult,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedExecutionResult should be validated and converted into
// a trusted ExecutionResult using NewExecutionResult constructor.
type UntrustedExecutionResult ExecutionResult

// NewExecutionResult creates a new instance of ExecutionResult.
// Construction ExecutionResult allowed only within the constructor.
//
// All errors indicate a valid ExecutionResult cannot be constructed from the input.
func NewExecutionResult(untrusted UntrustedExecutionResult) (*ExecutionResult, error) {
	if untrusted.PreviousResultID == ZeroID {
		return nil, fmt.Errorf("PreviousResultID must not be empty")
	}

	if untrusted.BlockID == ZeroID {
		return nil, fmt.Errorf("BlockID must not be empty")
	}

	if len(untrusted.Chunks) == 0 {
		return nil, fmt.Errorf("Chunks must not be empty")
	}

	if untrusted.ExecutionDataID == ZeroID {
		return nil, fmt.Errorf("ExecutionDataID must not be empty")
	}

	return &ExecutionResult{
		PreviousResultID: untrusted.PreviousResultID,
		BlockID:          untrusted.BlockID,
		Chunks:           untrusted.Chunks,
		ServiceEvents:    untrusted.ServiceEvents,
		ExecutionDataID:  untrusted.ExecutionDataID,
	}, nil
}

// NewRootExecutionResult creates a new instance of root ExecutionResult
// with empty PreviousResultID and ExecutionDataID fields.
// Construction ExecutionResult allowed only within the constructor.
//
// All errors indicate a valid root ExecutionResult cannot be constructed from the input.
func NewRootExecutionResult(untrusted UntrustedExecutionResult) (*ExecutionResult, error) {
	if untrusted.BlockID == ZeroID {
		return nil, fmt.Errorf("BlockID must not be empty")
	}

	if len(untrusted.Chunks) == 0 {
		return nil, fmt.Errorf("Chunks must not be empty")
	}

	return &ExecutionResult{
		PreviousResultID: untrusted.PreviousResultID,
		BlockID:          untrusted.BlockID,
		Chunks:           untrusted.Chunks,
		ServiceEvents:    untrusted.ServiceEvents,
		ExecutionDataID:  untrusted.ExecutionDataID,
	}, nil
}

// ID returns the hash of the execution result body
func (er ExecutionResult) ID() Identifier {
	return MakeID(er)
}

// ValidateChunksLength checks whether the number of chunks is zero.
//
// It returns false if the number of chunks is zero (invalid).
// By protocol definition, each ExecutionReceipt must contain at least one
// chunk (system chunk).
func (er ExecutionResult) ValidateChunksLength() bool {
	return er.Chunks.Len() != 0
}

// FinalStateCommitment returns the Execution Result's commitment to the final
// execution state of the block, i.e. the last chunk's output state.
//
// This function is side-effect free. The only possible error it returns is of type:
//   - ErrNoChunks: if there are no chunks (ExecutionResult is malformed)
func (er ExecutionResult) FinalStateCommitment() (StateCommitment, error) {
	if !er.ValidateChunksLength() {
		return DummyStateCommitment, ErrNoChunks
	}
	return er.Chunks[er.Chunks.Len()-1].EndState, nil
}

// InitialStateCommit returns a commitment to the execution state used as input
// for computing the block, i.e. the leading chunk's input state.
//
// This function is side-effect free. The only possible error it returns is of type
//   - ErrNoChunks: if there are no chunks (ExecutionResult is malformed)
func (er ExecutionResult) InitialStateCommit() (StateCommitment, error) {
	if !er.ValidateChunksLength() {
		return DummyStateCommitment, ErrNoChunks
	}
	return er.Chunks[0].StartState, nil
}

// SystemChunk is a system-generated chunk added to every block.
// It is always the final chunk in an execution result.
func (er ExecutionResult) SystemChunk() *Chunk {
	return er.Chunks[len(er.Chunks)-1]
}

// ServiceEventsByChunk returns the list of service events emitted during the given chunk.
func (er ExecutionResult) ServiceEventsByChunk(chunkIndex uint64) ServiceEventList {
	serviceEventCount := er.Chunks[chunkIndex].ServiceEventCount
	if serviceEventCount == 0 {
		return nil
	}

	startIndex := 0
	for i := range chunkIndex {
		startIndex += int(er.Chunks[i].ServiceEventCount)
	}
	return er.ServiceEvents[startIndex : startIndex+int(serviceEventCount)]
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
