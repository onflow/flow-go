package queue

import (
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/model/flow"
)

type node struct {
	Block    *execution.CompleteBlock
	Children []*node
}

// Queue is a fork-aware queue/tree of blocks for use in execution node, where parallel forks
// can be processed simultaneously. For fast lookup which is predicted to be common case
// all nodes are kept as one queue, which is expected to split into separate queues once
// a fork (multiple children) is reached.
// Note that this is not a thread-safe structure and external synchronisation is required
// to use in concurrent environment
type Queue struct {
	Head  *node
	Nodes map[flow.Identifier]*node
}

// traverse node children recursively and populate m
func traverse(node *node, m map[flow.Identifier]*node) {
	m[node.Block.Block.ID()] = node
	for _, node := range node.Children {
		traverse(node, m)
	}
}

func NewQueue(block *execution.CompleteBlock) *Queue {
	n := &node{
		Block:    block,
		Children: nil,
	}
	return &Queue{
		Head:  n,
		Nodes: map[flow.Identifier]*node{n.Block.Block.ID(): n},
	}
}


// rebuildQueue makes a new queue from a node which was already part of other queue
// and fills lookup cache
func rebuildQueue(n *node) *Queue {
	// rebuild map-cache
	cache := make(map[flow.Identifier]*node)
	traverse(n, cache)

	return &Queue{
		Head:  n,
		Nodes: cache,
	}
}

// Special case for removing single-childed head element
func dequeue(queue *Queue) *Queue {
	onlyChild := queue.Head.Children[0]

	cache := make(map[flow.Identifier]*node)

	//copy all but head caches
	for key, val := range queue.Nodes {
		if key != queue.Head.Block.Block.ID() {
			cache[key] = val
		}
	}
	return &Queue{
		Head: onlyChild,
		Nodes: cache,
	}
}

// TryAdd tries to add a new node to the queue and returns if the operation has been successful
func (q *Queue) TryAdd(block *execution.CompleteBlock) bool {
	n, ok := q.Nodes[block.Block.ParentID]
	if !ok {
		return false
	}
	newNode := &node{
		Block:    block,
		Children: nil,
	}
	n.Children = append(n.Children, newNode)
	q.Nodes[block.Block.ID()] = newNode
	return true
}

// Dismount removes the head element, returns it and it's children as new queues
func (q *Queue) Dismount() (*execution.CompleteBlock, []*Queue) {

	queues := make([]*Queue, len(q.Head.Children))
	if len(q.Head.Children) == 1 { //optimize for most common single-child case
		queues[0] = dequeue(q)
	} else {
		for i, child := range q.Head.Children {
			queues[i] = rebuildQueue(child)
		}
	}
	return q.Head.Block, queues
}
