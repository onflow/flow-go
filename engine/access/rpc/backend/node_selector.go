package backend

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// maxNodesCnt is the maximum number of nodes that will be contacted to complete an API request.
const maxNodesCnt = 3

// NodeSelector is an interface that represents the ability to select node identities that the access node is trying to reach.
// It encapsulates the internal logic of node selection and provides a way to change implementations for different types
// of nodes. Implementations of this interface should define the Next method, which returns the next node identity to be
// selected. HasNext checks if there is next node available.
type NodeSelector interface {
	Next() *flow.Identity
	HasNext() bool
}

// NodeSelectorFactory is a factory for creating node selectors based on factory configuration and node type.
type NodeSelectorFactory struct {
	circuitBreakerEnabled bool
}

// SelectNodes selects the configured number of node identities from the provided list of nodes
// and returns the node selector to iterate through them.
func (n *NodeSelectorFactory) SelectNodes(nodes flow.IdentityList) (NodeSelector, error) {
	var err error
	// If the circuit breaker is disabled, the legacy logic should be used, which selects only a specified number of nodes.
	if !n.circuitBreakerEnabled {
		nodes, err = nodes.Sample(maxNodesCnt)
		if err != nil {
			return nil, fmt.Errorf("sampling failed: %w", err)
		}
	}

	return &MainNodeSelector{
		nodes: nodes,
		index: 0,
	}, nil
}

// SelectCollectionNodes

var _ NodeSelector = (*MainNodeSelector)(nil)

// MainNodeSelector is a specific implementation of the node selector.
type MainNodeSelector struct {
	nodes flow.IdentityList
	index int
}

// HasNext returns true if next node is available.
func (e *MainNodeSelector) HasNext() bool {
	return e.index < len(e.nodes)
}

// Next returns the next node in the selector.
func (e *MainNodeSelector) Next() *flow.Identity {
	if e.index < len(e.nodes) {
		next := e.nodes[e.index]
		e.index++
		return next
	}
	return nil
}
