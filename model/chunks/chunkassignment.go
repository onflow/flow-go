package chunks

import (
	"github.com/onflow/flow-go/model/flow"
)

// Assignment is assignment map of the chunks to the list of the verifier nodes
type Assignment struct {
	verifiersForChunk map[uint64]map[flow.Identifier]struct{}
}

func NewAssignment() *Assignment {
	return &Assignment{
		verifiersForChunk: make(map[uint64]map[flow.Identifier]struct{}),
	}
}

// Verifiers returns the list of verifier nodes assigned to a chunk
func (a *Assignment) Verifiers(chunk *flow.Chunk) flow.IdentifierList {
	v := make([]flow.Identifier, 0)
	for id := range a.verifiersForChunk[chunk.Index] {
		v = append(v, id)
	}
	return v
}

// HasVerifier checks if a chunk is assigned to the given verifier
func (a *Assignment) HasVerifier(chunk *flow.Chunk, identifier flow.Identifier) bool {
	assignedVerifiers := a.verifiersForChunk[chunk.Index]
	_, isAssigned := assignedVerifiers[identifier]
	return isAssigned
}

// Add records the list of verifier nodes as the assigned verifiers of the chunk
// it returns an error if the list of verifiers is empty or contains duplicate ids
func (a *Assignment) Add(chunk *flow.Chunk, verifiers flow.IdentifierList) {
	// sorts verifiers list based on their identifier
	v := make(map[flow.Identifier]struct{})
	for _, id := range verifiers {
		v[id] = struct{}{}
	}
	a.verifiersForChunk[chunk.Index] = v
}

// ByNodeID returns the indices of all chunks assigned to the given verifierID
func (a *Assignment) ByNodeID(verifierID flow.Identifier) []uint64 {
	var chunks []uint64

	// iterates over pairs of (chunk index, assigned verifiers)
	for chunkIdx, assignedVerifiers := range a.verifiersForChunk {
		_, isAssigned := assignedVerifiers[verifierID]
		if isAssigned {
			chunks = append(chunks, chunkIdx)
		}
	}
	return chunks
}

// Len returns the number of chunks in the assignment
func (a *Assignment) Len() int {
	return len(a.verifiersForChunk)
}

// AssignmentDataPack
//
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
