package chunks

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
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

	//strings.Join(hexStrings)
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
	expected   []byte
	computed   []byte
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
func NewCFNonMatchingFinalState(expected []byte, computed []byte, chInx uint64, execResID flow.Identifier) *CFNonMatchingFinalState {
	return &CFNonMatchingFinalState{expected: expected,
		computed:   computed,
		chunkIndex: chInx,
		execResID:  execResID}
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
