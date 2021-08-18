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
	txID       flow.Identifier // very first transaction inside the chunk that required this register
}

func (cf CFMissingRegisterTouch) String() string {
	hexStrings := make([]string, len(cf.regsterIDs))
	for i, s := range cf.regsterIDs {
		hexStrings[i] = hex.EncodeToString([]byte(s))
	}

	return fmt.Sprintf("at least one register touch was missing inside the chunk data package that was needed while running transactions of chunk %d of result %s (tx hash of one of them: %s), hex-encoded register ids: %s", cf.chunkIndex, cf.execResID.String(), cf.txID.String(), hexStrings)
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
func NewCFMissingRegisterTouch(regsterIDs []string, chInx uint64, execResID flow.Identifier, txID flow.Identifier) *CFMissingRegisterTouch {
	return &CFMissingRegisterTouch{regsterIDs: regsterIDs,
		chunkIndex: chInx,
		execResID:  execResID,
		txID:       txID}
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
	eventIDs   flow.IdentifierList
}

func NewCFInvalidEventsCollection(expected flow.Identifier, computed flow.Identifier, chInx uint64, execResID flow.Identifier, events flow.EventsList) *CFInvalidEventsCollection {
	return &CFInvalidEventsCollection{
		expected:   expected,
		computed:   computed,
		chunkIndex: chInx,
		resultID:   execResID,
		eventIDs:   flow.GetIDs(events),
	}
}

func (c *CFInvalidEventsCollection) ChunkIndex() uint64 {
	return c.chunkIndex
}

func (c *CFInvalidEventsCollection) ExecutionResultID() flow.Identifier {
	return c.resultID
}

func (c *CFInvalidEventsCollection) String() string {
	return fmt.Sprintf("events collection hash differs, got %x expected %x for chunk %d with result ID %s, events IDs: %v", c.computed, c.expected,
		c.chunkIndex, c.resultID, c.eventIDs)
}

// CFInvalidServiceEventsEmitted is returned when service events are different from the chunk's one
type CFInvalidServiceEventsEmitted struct {
	expected   flow.ServiceEventList
	computed   flow.ServiceEventList
	chunkIndex uint64
	resultID   flow.Identifier
}

func CFInvalidServiceSystemEventsEmitted(expected flow.ServiceEventList, computed flow.ServiceEventList, chInx uint64, execResID flow.Identifier) *CFInvalidServiceEventsEmitted {
	return &CFInvalidServiceEventsEmitted{
		expected:   expected,
		computed:   computed,
		chunkIndex: chInx,
		resultID:   execResID,
	}
}

func (c *CFInvalidServiceEventsEmitted) ChunkIndex() uint64 {
	return c.chunkIndex
}

func (c *CFInvalidServiceEventsEmitted) ExecutionResultID() flow.Identifier {
	return c.resultID
}

func (c *CFInvalidServiceEventsEmitted) String() string {
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
