package backend

import "github.com/onflow/flow-go/model/flow"

// maxExecutionNodesCnt is the maximum number of execution nodes that will be contacted to complete an execution API request.
const maxExecutionNodesCnt = 3

// maxCollectionNodesCnt is the maximum number of collection nodes that will be contacted to complete a collection API request.
const maxCollectionNodesCnt = 3

// NodeSelector is an interface that represents the ability to select node identities that the access node is trying to reach.
// It encapsulates the internal logic of node selection and provides a way to change implementations for different types
// of nodes. Implementations of this interface should define the Next method, which returns the next node identity to be
// selected.
type NodeSelector interface {
	Next() *flow.Identity
}

// NodeSelectorFactory is a factory for creating node selectors based on factory configuration and node type.
type NodeSelectorFactory struct {
	circuitBreakerEnabled bool
}

// SelectExecutionNodes selects the configured number of execution node identities from the provided list of execution nodes
// and returns an execution node selector to iterate through them.
func (n *NodeSelectorFactory) SelectExecutionNodes(executionNodes flow.IdentityList) NodeSelector {
	// If the circuit breaker is disabled, the legacy logic should be used, which selects only a specified number of nodes.
	if !n.circuitBreakerEnabled {
		executionNodes = executionNodes.Sample(maxExecutionNodesCnt)
	}

	return &ExecutionNodeSelector{
		nodes: executionNodes,
		index: 0,
	}
}

// SelectCollectionNodes selects the configured number of collection node identities from the provided list of collection nodes
// and returns a collection node selector to iterate through them.
func (n *NodeSelectorFactory) SelectCollectionNodes(collectionNodes flow.IdentityList) NodeSelector {
	// If the circuit breaker is disabled, the legacy logic should be used, which selects only a specified number of nodes.
	if !n.circuitBreakerEnabled {
		collectionNodes = collectionNodes.Sample(maxCollectionNodesCnt)
	}

	return &CollectionNodeSelector{
		nodes: collectionNodes,
		index: 0,
	}
}

var _ NodeSelector = (*ExecutionNodeSelector)(nil)

// ExecutionNodeSelector is a specific implementation of an execution node selector.
type ExecutionNodeSelector struct {
	nodes flow.IdentityList
	index int
}

// Next returns the next execution node in the selector.
func (e *ExecutionNodeSelector) Next() *flow.Identity {
	if e.index < len(e.nodes) {
		next := e.nodes[e.index]
		e.index++
		return next
	}
	return nil
}

var _ NodeSelector = (*CollectionNodeSelector)(nil)

// CollectionNodeSelector is a specific implementation of a collection node selector.
type CollectionNodeSelector struct {
	nodes flow.IdentityList
	index int
}

// Next returns the next collection node in the selector.
func (c *CollectionNodeSelector) Next() *flow.Identity {
	if c.index < len(c.nodes) {
		next := c.nodes[c.index]
		c.index++
		return next
	}
	return nil
}
