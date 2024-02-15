package chunks

// This file contains the ChunkFaultErrors returned during chunk verification of ExecutionData.

import (
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/flow"
)

// CFExecutionDataBlockIDMismatch is returned when the block ID contained in the execution data
// root is different from chunk's block ID
type CFExecutionDataBlockIDMismatch struct {
	chunkIndex               uint64          // chunk's index
	execResID                flow.Identifier // chunk's ExecutionResult identifier
	executionDataRootBlockID flow.Identifier // blockID from chunk's ExecutionDataRoot
	chunkBlockID             flow.Identifier // chunk's blockID
}

var _ ChunkFaultError = (*CFExecutionDataBlockIDMismatch)(nil)

func (cf CFExecutionDataBlockIDMismatch) String() string {
	return fmt.Sprintf("execution data root's block ID (%s) is different than chunk's block ID (%s) for chunk %d with result ID %s",
		cf.executionDataRootBlockID, cf.chunkBlockID, cf.chunkIndex, cf.execResID.String())
}

func (cf CFExecutionDataBlockIDMismatch) Error() string {
	return cf.String()
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFExecutionDataBlockIDMismatch) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFExecutionDataBlockIDMismatch) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFExecutionDataBlockIDMismatch creates a new instance of Chunk Fault (ExecutionDataBlockIDMismatch)
func NewCFExecutionDataBlockIDMismatch(
	executionDataRootBlockID flow.Identifier,
	chunkBlockID flow.Identifier,
	chInx uint64,
	execResID flow.Identifier,
) *CFExecutionDataBlockIDMismatch {
	return &CFExecutionDataBlockIDMismatch{
		chunkIndex:               chInx,
		execResID:                execResID,
		executionDataRootBlockID: executionDataRootBlockID,
		chunkBlockID:             chunkBlockID,
	}
}

// CFExecutionDataChunksLengthMismatch is returned when execution data chunks list has different length than number of chunks for a block
type CFExecutionDataChunksLengthMismatch struct {
	chunkIndex                     uint64          // chunk's index
	execResID                      flow.Identifier // chunk's ExecutionResult identifier
	executionDataRootChunkLength   int             // number of ChunkExecutionDataIDs in ExecutionDataRoot
	executionResultChunkListLength int             // number of chunks in ExecutionResult
}

var _ ChunkFaultError = (*CFExecutionDataChunksLengthMismatch)(nil)

func (cf CFExecutionDataChunksLengthMismatch) String() string {
	return fmt.Sprintf("execution data root chunk length (%d) is different than execution result chunk list length (%d) for chunk %d with result ID %s",
		cf.executionDataRootChunkLength, cf.executionResultChunkListLength, cf.chunkIndex, cf.execResID.String())
}

func (cf CFExecutionDataChunksLengthMismatch) Error() string {
	return cf.String()
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFExecutionDataChunksLengthMismatch) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFExecutionDataChunksLengthMismatch) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFExecutionDataChunksLengthMismatch creates a new instance of Chunk Fault (ExecutionDataBlockIDMismatch)
func NewCFExecutionDataChunksLengthMismatch(
	executionDataRootChunkLength int,
	executionResultChunkListLength int,
	chInx uint64,
	execResID flow.Identifier,
) *CFExecutionDataChunksLengthMismatch {
	return &CFExecutionDataChunksLengthMismatch{
		chunkIndex:                     chInx,
		execResID:                      execResID,
		executionDataRootChunkLength:   executionDataRootChunkLength,
		executionResultChunkListLength: executionResultChunkListLength,
	}
}

// CFExecutionDataInvalidChunkCID is returned when execution data chunk's CID is different from computed
type CFExecutionDataInvalidChunkCID struct {
	chunkIndex                uint64          // chunk's index
	execResID                 flow.Identifier // chunk's ExecutionResult identifier
	executionDataRootChunkCID cid.Cid         // ExecutionDataRoot's CID for the chunk
	computedChunkCID          cid.Cid         // computed CID for the chunk
}

var _ ChunkFaultError = (*CFExecutionDataInvalidChunkCID)(nil)

func (cf CFExecutionDataInvalidChunkCID) String() string {
	return fmt.Sprintf("execution data chunk CID (%s) is different than computed (%s) for chunk %d with result ID %s",
		cf.executionDataRootChunkCID, cf.computedChunkCID, cf.chunkIndex, cf.execResID.String())
}

func (cf CFExecutionDataInvalidChunkCID) Error() string {
	return cf.String()
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFExecutionDataInvalidChunkCID) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFExecutionDataInvalidChunkCID) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFExecutionDataInvalidChunkCID creates a new instance of Chunk Fault (NewCFExecutionDataInvalidChunkCID)
func NewCFExecutionDataInvalidChunkCID(
	executionDataRootChunkCID cid.Cid,
	computedChunkCID cid.Cid,
	chInx uint64,
	execResID flow.Identifier,
) *CFExecutionDataInvalidChunkCID {
	return &CFExecutionDataInvalidChunkCID{
		chunkIndex:                chInx,
		execResID:                 execResID,
		executionDataRootChunkCID: executionDataRootChunkCID,
		computedChunkCID:          computedChunkCID,
	}
}

// CFInvalidExecutionDataID is returned when ExecutionResult's ExecutionDataID is different from computed
type CFInvalidExecutionDataID struct {
	chunkIndex              uint64          // chunk's index
	execResID               flow.Identifier // chunk's ExecutionResult identifier
	erExecutionDataID       flow.Identifier // ExecutionResult's ExecutionDataID
	computedExecutionDataID flow.Identifier // computed ExecutionDataID
}

var _ ChunkFaultError = (*CFInvalidExecutionDataID)(nil)

func (cf CFInvalidExecutionDataID) String() string {
	return fmt.Sprintf("execution data ID (%s) is different than computed (%s) for chunk %d with result ID %s",
		cf.erExecutionDataID, cf.computedExecutionDataID, cf.chunkIndex, cf.execResID.String())
}

func (cf CFInvalidExecutionDataID) Error() string {
	return cf.String()
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFInvalidExecutionDataID) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFInvalidExecutionDataID) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFInvalidExecutionDataID creates a new instance of Chunk Fault (CFInvalidExecutionDataID)
func NewCFInvalidExecutionDataID(
	erExecutionDataID flow.Identifier,
	computedExecutionDataID flow.Identifier,
	chInx uint64,
	execResID flow.Identifier,
) *CFInvalidExecutionDataID {
	return &CFInvalidExecutionDataID{
		chunkIndex:              chInx,
		execResID:               execResID,
		erExecutionDataID:       erExecutionDataID,
		computedExecutionDataID: computedExecutionDataID,
	}
}
