package backend

import "github.com/onflow/flow-go/model/flow"

// maxExecutionNodesCnt is the max number of execution nodes that will be contacted to complete an execution api request
const maxExecutionNodesCnt = 3

const collectionNodesToTry = 3

type NodeSelector interface {
	Next() *flow.Identity
}

type NodeSelectorFactory struct {
	circuitBreakerEnabled bool
}

func (n *NodeSelectorFactory) SelectExecutionNodes(nodes flow.IdentityList) NodeSelector {
	if !n.circuitBreakerEnabled {
		nodes = nodes.Sample(maxExecutionNodesCnt)
	}

	return &ExecutionNodeIterator{
		nodes: nodes,
		index: 0,
	}
}

func (n *NodeSelectorFactory) SelectCollectionNodes(nodes flow.IdentityList) NodeSelector {
	if !n.circuitBreakerEnabled {
		nodes = nodes.Sample(collectionNodesToTry)
	}

	return &CollectionNodeIterator{
		nodes: nodes,
		index: 0,
	}
}

var _ NodeSelector = (*ExecutionNodeIterator)(nil)

type ExecutionNodeIterator struct {
	nodes flow.IdentityList
	index int
}

func (e *ExecutionNodeIterator) Next() *flow.Identity {
	if e.index < len(e.nodes) {
		next := e.nodes[e.index]
		e.index++
		return next
	}
	return nil
}

var _ NodeSelector = (*CollectionNodeIterator)(nil)

type CollectionNodeIterator struct {
	nodes flow.IdentityList
	index int
}

func (c *CollectionNodeIterator) Next() *flow.Identity {
	if c.index < len(c.nodes) {
		next := c.nodes[c.index]
		c.index++
		return next
	}
	return nil
}
