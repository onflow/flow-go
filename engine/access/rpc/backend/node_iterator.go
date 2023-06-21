package backend

import "github.com/onflow/flow-go/model/flow"

// maxExecutionNodesCnt is the max number of execution nodes that will be contacted to complete an execution api request
const maxExecutionNodesCnt = 3

const collectionNodesToTry = 3

type NodeIterator interface {
	Next() *flow.Identity
}

type NodeIteratorFactory interface {
	CreateNodeIterator(nodes flow.IdentityList) NodeIterator
}

var _ NodeIteratorFactory = (*ExecutionNodeIteratorFactory)(nil)
var _ NodeIterator = (*ExecutionNodeIterator)(nil)
var _ NodeIteratorFactory = (*CollectionNodeIteratorFactory)(nil)
var _ NodeIterator = (*CollectionNodeIterator)(nil)

type ExecutionNodeIteratorFactory struct {
	circuitBreakerEnabled bool
}

func (e *ExecutionNodeIteratorFactory) CreateNodeIterator(nodes flow.IdentityList) NodeIterator {
	if !e.circuitBreakerEnabled {
		nodes = nodes.Sample(maxExecutionNodesCnt)
	}

	return &ExecutionNodeIterator{
		nodes: nodes,
		index: 0,
	}
}

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

type CollectionNodeIteratorFactory struct {
	circuitBreakerEnabled bool
}

func (c *CollectionNodeIteratorFactory) CreateNodeIterator(nodes flow.IdentityList) NodeIterator {
	if !c.circuitBreakerEnabled {
		nodes = nodes.Sample(collectionNodesToTry)
	}

	return &CollectionNodeIterator{
		nodes: nodes,
		index: 0,
	}
}

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
