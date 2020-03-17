package chunkassignment

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Assignment is assignment map of the chunks to the list of the verifier nodes
type Assignment struct {
	id    flow.Identifier // unique identifier for this assignment
	table map[uint64]map[flow.Identifier]struct{}
}

func NewAssignment(id flow.Identifier) *Assignment {
	return &Assignment{
		id:    id,
		table: make(map[uint64]map[flow.Identifier]struct{}),
	}
}

// ID returns the unique identifier for assignment
func (a *Assignment) ID() flow.Identifier {
	return a.id
}

// Checksum returns the checksum of the assignment
func (a *Assignment) Checksum() flow.Identifier {
	return flow.MakeID(a)
}

// Verifiers returns the list of verifier nodes assigned to a chunk
func (a *Assignment) Verifiers(chunk *flow.Chunk) flow.IdentifierList {
	v := make([]flow.Identifier, 0)
	for id := range a.table[chunk.Index] {
		v = append(v, id)
	}
	return v
}

// Add records the list of verifier nodes as the assigned verifiers of the chunk
// it returns an error if the list of verifiers is empty or contains duplicate ids
func (a *Assignment) Add(chunk *flow.Chunk, verifiers flow.IdentifierList) {
	// sorts verifiers list based on their identifier
	v := make(map[flow.Identifier]struct{})
	for _, id := range verifiers {
		v[id] = struct{}{}
	}
	a.table[chunk.Index] = v
}

func (a *Assignment) ByNodeID(this flow.Identifier) []uint64 {
	var chunks []uint64

	// iterates over pairs of (chunk, assigned verifiers)
	for c, vList := range a.table {
		// for each chunk iterates over identifiers
		// of its assigned verifier nodes
		for id := range vList {
			if id == this {
				chunks = append(chunks, c)
			}
		}
	}
	return chunks
}
