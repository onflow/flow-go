package chunks

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

var ErrUnknownChunkIndex = errors.New("verifier assignment for invalid chunk requested")

// Assignment is an immutable list that, for each chunk (in order of chunk.Index),
// records the set of verifier nodes that are assigned to verify that chunk.
// Assignments are only constructed using AssignmentBuilder, and cannot be modified after construction.
type Assignment struct {
	verifiersForChunk []map[flow.Identifier]struct{}
}

// AssignmentBuilder is a helper for constructing a single new Assignment,
// and should be discarded after calling `Build()`.
// AssignmentBuilder is not safe for concurrent use by multiple goroutines.
type AssignmentBuilder struct {
	verifiersForChunk []map[flow.Identifier]struct{}
}

func NewAssignmentBuilder() *AssignmentBuilder {
	return &AssignmentBuilder{
		verifiersForChunk: make([]map[flow.Identifier]struct{}, 0, 1),
	}
}

// Build constructs and returns the immutable assignment. The AssignmentBuilder
// should be discarded after this call, and further method calls will panic.
func (a *AssignmentBuilder) Build() *Assignment {
	if a.verifiersForChunk == nil {
		panic("method `AssignmentBuilder.Build` has previously been called - do not reuse AssignmentBuilder")
	}
	assignment := &Assignment{verifiersForChunk: a.verifiersForChunk}
	a.verifiersForChunk = nil // revoke builder's reference, to prevent modification of assignment
	return assignment
}

// Verifiers returns the list of verifier nodes assigned to a chunk. The protocol mandates
// that for each chunk in a block, a verifier assignment exists (though it may be empty) and
// that each block must at least have one chunk.
// Errors: ErrUnknownChunkIndex if the chunk index is not present in the assignment
func (a *Assignment) Verifiers(chunkIdx uint64) (flow.IdentifierList, error) {
	if chunkIdx >= uint64(len(a.verifiersForChunk)) {
		return nil, ErrUnknownChunkIndex
	}
	assignedVerifiers := a.verifiersForChunk[chunkIdx]
	v := make([]flow.Identifier, 0, len(assignedVerifiers))
	for id := range a.verifiersForChunk[chunkIdx] {
		v = append(v, id)
	}
	return v, nil
}

// HasVerifier checks if a chunk is assigned to the given verifier
// Errors: ErrUnknownChunkIndex if the chunk index is not present in the assignment
func (a *Assignment) HasVerifier(chunkIdx uint64, identifier flow.Identifier) (bool, error) {
	if chunkIdx >= uint64(len(a.verifiersForChunk)) {
		return false, ErrUnknownChunkIndex
	}
	assignedVerifiers := a.verifiersForChunk[chunkIdx]
	_, isAssigned := assignedVerifiers[identifier]
	return isAssigned, nil
}

// Add records the list of verifier nodes as the assigned verifiers of the chunk.
// Requires chunks to be added in order of their Index (starting at 0 and increasing
// by 1 with each addition); otherwise an exception is returned.
func (a *AssignmentBuilder) Add(chunkIdx uint64, verifiers flow.IdentifierList) error {
	if a.verifiersForChunk == nil {
		panic("method `AssignmentBuilder.Build` has previously been called - do not reuse AssignmentBuilder")
	}
	if chunkIdx != uint64(len(a.verifiersForChunk)) {
		return fmt.Errorf("chunk added out of order, got index %v but expecting %v", chunkIdx, len(a.verifiersForChunk))
	}
	// Formally, the flow protocol mandates that the same verifier is not assigned
	// repeatedly to the same chunk (as this would weaken the protocol's security).
	vs := verifiers.Lookup()
	if len(vs) != len(verifiers) {
		return fmt.Errorf("repeated assignment of the same verifier to the same chunk is a violation of protocol rules")
	}
	a.verifiersForChunk = append(a.verifiersForChunk, vs)
	return nil
}

// ByNodeID returns the indices of all chunks assigned to the given verifierID
func (a *Assignment) ByNodeID(verifierID flow.Identifier) []uint64 {
	var chunks []uint64

	// iterates over pairs of (chunk index, assigned verifiers)
	for chunkIdx, assignedVerifiers := range a.verifiersForChunk {
		_, isAssigned := assignedVerifiers[verifierID]
		if isAssigned {
			chunks = append(chunks, uint64(chunkIdx))
		}
	}
	return chunks
}

// Len returns the number of chunks in the assignment
func (a *Assignment) Len() int {
	return len(a.verifiersForChunk)
}

// AssignmentDataPack provides a storable representation of chunk assignments on
// mempool
type AssignmentDataPack struct {
	assignment  *Assignment
	fingerprint flow.Identifier
}

// NewAssignmentDataPack casts an assignment and its fingerprint into an assignment data pack
func NewAssignmentDataPack(fingerprint flow.Identifier, assignment *Assignment) *AssignmentDataPack {
	return &AssignmentDataPack{
		assignment:  assignment,
		fingerprint: fingerprint,
	}
}

// ID returns the unique identifier for assignment data pack
func (a *AssignmentDataPack) ID() flow.Identifier {
	return a.fingerprint
}

// Checksum returns the checksum of the assignment data pack
func (a *AssignmentDataPack) Checksum() flow.Identifier {
	return flow.MakeID(a)
}

// Assignment returns the assignment part of the assignment data pack
func (a *AssignmentDataPack) Assignment() *Assignment {
	return a.assignment
}
