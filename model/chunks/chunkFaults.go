package chunks

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkFault holds information about a fault that is found while
// verifying a chunk
type ChunkFault interface {
	ChunkIndex() uint64
	ExecutionResultID() flow.Identifier
	String() string
}

// CFMissingRegisterTouch is returned when a register touch is missing (read or update)
type CFMissingRegisterTouch struct {
	regsterIDs []string
	chunkIndex uint64
	execResID  flow.Identifier
}

func (cf CFMissingRegisterTouch) String() string {
	hexStrings := make([]string, len(cf.regsterIDs))
	for i, s := range cf.regsterIDs {
		hexStrings[i] = hex.EncodeToString([]byte(s))
	}

	return fmt.Sprint("at least one register touch was missing inside the chunk data package that was needed while running transactions (hex-encoded): ", hexStrings)
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFMissingRegisterTouch) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFMissingRegisterTouch) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFMissingRegisterTouch creates a new instance of Chunk Fault (MissingRegisterTouch)
func NewCFMissingRegisterTouch(regsterIDs []string, chInx uint64, execResID flow.Identifier) *CFMissingRegisterTouch {
	return &CFMissingRegisterTouch{regsterIDs: regsterIDs,
		chunkIndex: chInx,
		execResID:  execResID}
}

// CFNonMatchingFinalState is returned when the computed final state commitment
// (applying chunk register updates to the partial trie) doesn't match the one provided by the chunk
type CFNonMatchingFinalState struct {
	expected   flow.StateCommitment
	computed   flow.StateCommitment
	chunkIndex uint64
	execResID  flow.Identifier
}

func (cf CFNonMatchingFinalState) String() string {
	return fmt.Sprintf("final state commitment doesn't match, expected [%x] but computed [%x]", cf.expected, cf.computed)
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFNonMatchingFinalState) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFNonMatchingFinalState) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFNonMatchingFinalState creates a new instance of Chunk Fault (NonMatchingFinalState)
func NewCFNonMatchingFinalState(expected flow.StateCommitment, computed flow.StateCommitment, chInx uint64, execResID flow.Identifier) *CFNonMatchingFinalState {
	return &CFNonMatchingFinalState{expected: expected,
		computed:   computed,
		chunkIndex: chInx,
		execResID:  execResID}
}

// CFInvalidEventsCollection is returned when computed events collection hash is different from the chunk's one
type CFInvalidEventsCollection struct {
	expected   flow.Identifier
	computed   flow.Identifier
	chunkIndex uint64
	resultID   flow.Identifier
}

func NewCFInvalidEventsCollection(expected flow.Identifier, computed flow.Identifier, chInx uint64, execResID flow.Identifier) *CFInvalidEventsCollection {
	return &CFInvalidEventsCollection{
		expected:   expected,
		computed:   computed,
		chunkIndex: chInx,
		resultID:   execResID,
	}
}

func (c *CFInvalidEventsCollection) ChunkIndex() uint64 {
	return c.chunkIndex
}

func (c *CFInvalidEventsCollection) ExecutionResultID() flow.Identifier {
	return c.resultID
}

func (c *CFInvalidEventsCollection) String() string {
	return fmt.Sprintf("events collection hash differs, got %x expected %x for chunk %d with result ID %s", c.computed, c.expected, c.chunkIndex, c.resultID)
}

// CFInvalidSystemEventsEmitted is returned when service events are different from the chunk's one
type CFInvalidSystemEventsEmitted struct {
	expected   flow.ServiceEventsList
	computed   flow.ServiceEventsList
	chunkIndex uint64
	resultID   flow.Identifier
}

func CFInvalidServiceSystemEventsEmitted(expected flow.ServiceEventsList, computed flow.ServiceEventsList, chInx uint64, execResID flow.Identifier) *CFInvalidSystemEventsEmitted {
	return &CFInvalidSystemEventsEmitted{
		expected:   expected,
		computed:   computed,
		chunkIndex: chInx,
		resultID:   execResID,
	}
}

func (c *CFInvalidSystemEventsEmitted) ChunkIndex() uint64 {
	return c.chunkIndex
}

func (c *CFInvalidSystemEventsEmitted) ExecutionResultID() flow.Identifier {
	return c.resultID
}

func (c *CFInvalidSystemEventsEmitted) String() string {
	return fmt.Sprintf("service events differs, got [%s] expected [%s] for chunk %d with result ID %s", c.computed, c.expected, c.chunkIndex, c.resultID)
}

// CFInvalidVerifiableChunk is returned when a verifiable chunk is invalid
// this includes cases that code fails to construct a partial trie,
// collection hashes doesn't match
// TODO break this into more detailed ones as we progress
type CFInvalidVerifiableChunk struct {
	reason     string
	details    error
	chunkIndex uint64
	execResID  flow.Identifier
}

func (cf CFInvalidVerifiableChunk) String() string {
	return fmt.Sprint("invalid verifiable chunk due to ", cf.reason, cf.details.Error())
}

// ChunkIndex returns chunk index of the faulty chunk
func (cf CFInvalidVerifiableChunk) ChunkIndex() uint64 {
	return cf.chunkIndex
}

// ExecutionResultID returns the execution result identifier including the faulty chunk
func (cf CFInvalidVerifiableChunk) ExecutionResultID() flow.Identifier {
	return cf.execResID
}

// NewCFInvalidVerifiableChunk creates a new instance of Chunk Fault (InvalidVerifiableChunk)
func NewCFInvalidVerifiableChunk(reason string, err error, chInx uint64, execResID flow.Identifier) *CFInvalidVerifiableChunk {
	return &CFInvalidVerifiableChunk{reason: reason,
		details:    err,
		chunkIndex: chInx,
		execResID:  execResID}
}
