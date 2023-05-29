package forks

import "github.com/onflow/flow-go/model/flow"

type QuorumCertificate interface {
	// BlockID returns the identifier for the block that this QC is poi
	BlockID() flow.Identifier
	View() uint64
}

type Block interface {
	// VertexID returns the vertex's ID (in most cases its hash)
	BlockID() flow.Identifier
	// Level returns the vertex's level
	View() uint64
	// Parent returns the parent's (level, ID)
	Parent() (flow.Identifier, uint64)
}
