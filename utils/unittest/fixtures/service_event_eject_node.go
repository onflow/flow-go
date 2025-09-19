package fixtures

import "github.com/onflow/flow-go/model/flow"

// EjectNode is the default options factory for [flow.EjectNode] generation.
var EjectNode ejectNodeFactory

type ejectNodeFactory struct{}

type EjectNodeOption func(*EjectNodeGenerator, *flow.EjectNode)

// WithNodeID is an option that sets the `NodeID` of the node to be ejected.
func (f ejectNodeFactory) WithNodeID(nodeID flow.Identifier) EjectNodeOption {
	return func(g *EjectNodeGenerator, eject *flow.EjectNode) {
		eject.NodeID = nodeID
	}
}

// EjectNodeGenerator generates node ejection events with consistent randomness.
type EjectNodeGenerator struct {
	ejectNodeFactory

	identifiers *IdentifierGenerator
}

// NewEjectNodeGenerator creates a new EjectNodeGenerator.
func NewEjectNodeGenerator(
	identifiers *IdentifierGenerator,
) *EjectNodeGenerator {
	return &EjectNodeGenerator{
		identifiers: identifiers,
	}
}

// Fixture generates a [flow.EjectNode] with random data based on the provided options.
func (g *EjectNodeGenerator) Fixture(opts ...EjectNodeOption) *flow.EjectNode {
	eject := &flow.EjectNode{
		NodeID: g.identifiers.Fixture(),
	}

	for _, opt := range opts {
		opt(g, eject)
	}

	return eject
}

// List generates a list of [flow.EjectNode].
func (g *EjectNodeGenerator) List(n int, opts ...EjectNodeOption) []*flow.EjectNode {
	ejects := make([]*flow.EjectNode, n)
	for i := range n {
		ejects[i] = g.Fixture(opts...)
	}
	return ejects
}
