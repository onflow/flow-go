package chunks

import (
	"fmt"
)

// ErrIncompleteVerifiableChunk returned when a verifiable chunk are missing fields.
type ErrIncompleteVerifiableChunk struct {
	missing []string // the missing fields
}

func (e ErrIncompleteVerifiableChunk) Error() string {
	return fmt.Sprint("incomplete verifiable chunk, missing fields: ", e.missing)
}

// Is return true if the error type matches
func (e ErrIncompleteVerifiableChunk) Is(other error) bool {
	_, ok := other.(ErrIncompleteVerifiableChunk)
	return ok
}

// ErrInvalidVerifiableChunk returned when a verifiable chunk is invalid
// this includes cases that code fails to construct a partial trie,
// collection hashes doesn't match
type ErrInvalidVerifiableChunk struct {
	reason  string
	details error
}

func (e ErrInvalidVerifiableChunk) Error() string {
	return fmt.Sprint("invalid verifiable chunk due to ", e.reason, e.details.Error())
}

// Is return true if the error type matches
func (e ErrInvalidVerifiableChunk) Is(other error) bool {
	_, ok := other.(ErrInvalidVerifiableChunk)
	return ok
}

// ErrMissingRegisterTouch returned when a register touch is missing (read or update)
type ErrMissingRegisterTouch struct {
	regsterIDs []string
}

func (e ErrMissingRegisterTouch) Error() string {
	return fmt.Sprint("atleast one register touch was missing inside the chunk data package that was needed while running transactions: ", e.regsterIDs)
}

// Is return true if the error type matches
func (e ErrMissingRegisterTouch) Is(other error) bool {
	_, ok := other.(ErrMissingRegisterTouch)
	return ok
}

// ErrNonMatchingFinalState returned when the computed final state commitment
// (applying chunk register updates to the partial trie) doesn't match the one provided by the chunk
type ErrNonMatchingFinalState struct {
	expected []byte
	computed []byte
}

func (e ErrNonMatchingFinalState) Error() string {
	return fmt.Sprintf("final state commitment doesn't match, expected [%x] but computed [%x]", e.expected, e.computed)
}

// Is return true if the error type matches
func (e ErrNonMatchingFinalState) Is(other error) bool {
	_, ok := other.(ErrNonMatchingFinalState)
	return ok
}
