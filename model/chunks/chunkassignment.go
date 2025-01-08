package chunks

import (
	"github.com/onflow/flow-go/model/flow"
)

// Assignment is an immutable list that, for each chunk (in order),
// records the set of verifier nodes that are assigned to verify that chunk.
type Assignment struct {
	verifiersForChunk []map[flow.Identifier]struct{}
}

// AssignmentBuilder is a helper to create a new Assignment
type AssignmentBuilder struct {
	verifiersForChunk []map[flow.Identifier]struct{}
}

func NewAssignmentBuilder() *AssignmentBuilder {
	return &AssignmentBuilder{
		verifiersForChunk: make([]map[flow.Identifier]struct{}, 0, 1),
	}
}

func (a *AssignmentBuilder) Build() *Assignment {
	// TODO is this correct?
	// the underlying data can still be modified by the Builder,
	// but the Builder ideally shouldn't be kept around or reused, so it should be sufficient
	return &Assignment{
		verifiersForChunk: a.verifiersForChunk,
	}
}

// Verifiers returns the list of verifier nodes assigned to a chunk
func (a *Assignment) Verifiers(chunk *flow.Chunk) flow.IdentifierList {
	v := make([]flow.Identifier, 0)
	if chunk.Index >= uint64(len(a.verifiersForChunk)) {
		// the chunk does not exist, so it has no verifiers
		return v
	}
	for id := range a.verifiersForChunk[chunk.Index] {
		v = append(v, id)
	}
	return v
}

// HasVerifier checks if a chunk is assigned to the given verifier
// TODO: method should probably error if chunk has unknown index
func (a *Assignment) HasVerifier(chunk *flow.Chunk, identifier flow.Identifier) bool {
	if chunk.Index >= uint64(len(a.verifiersForChunk)) {
		// is verifier assigned to this chunk?
		// No, because we only assign verifiers to existing chunks
		return false
	}
	assignedVerifiers := a.verifiersForChunk[chunk.Index]
	_, isAssigned := assignedVerifiers[identifier]
	return isAssigned
}

// Add records the list of verifier nodes as the assigned verifiers of the chunk.
// It returns an error if the list of verifiers is empty or contains duplicate ids
func (a *AssignmentBuilder) Add(chunk *flow.Chunk, verifiers flow.IdentifierList) {
	if chunk.Index != uint64(len(a.verifiersForChunk)) {
		panic("chunks added out of order")
	}
	// sorts verifiers list based on their identifier
	v := make(map[flow.Identifier]struct{})
	for _, id := range verifiers {
		v[id] = struct{}{}
	}
	a.verifiersForChunk = append(a.verifiersForChunk, v)
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
