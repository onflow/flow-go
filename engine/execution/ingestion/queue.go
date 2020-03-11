package ingestion

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Node struct {
	CompleteBlock *execution.CompleteBlock
	Children      []*Node
}

// Queue is a fork-aware queue/tree of blocks for use in execution Node, where parallel forks
// can be processed simultaneously. For fast lookup which is predicted to be common case
// all nodes are kept as one queue, which is expected to split into separate queues once
// a fork (multiple children) is reached.
// Note that this is not a thread-safe structure and external synchronisation is required
// to use in concurrent environment
type Queue struct {
	Head  *Node
	Nodes map[flow.Identifier]*Node
}

// Make queue an entity so it can be stored in mempool

func (q *Queue) ID() flow.Identifier {
	return q.Head.CompleteBlock.Block.ID()
}

func (q *Queue) Checksum() flow.Identifier {
	return q.Head.CompleteBlock.Block.Checksum()
}

// traverse Node children recursively and populate m
func traverse(node *Node, m map[flow.Identifier]*Node) {
	m[node.CompleteBlock.Block.ID()] = node
	for _, node := range node.Children {
		traverse(node, m)
	}
}

func NewQueue(completeBlock *execution.CompleteBlock) *Queue {
	n := &Node{
		CompleteBlock: completeBlock,
		Children:      nil,
	}
	return &Queue{
		Head:  n,
		Nodes: map[flow.Identifier]*Node{n.CompleteBlock.Block.ID(): n},
	}
}

// rebuildQueue makes a new queue from a Node which was already part of other queue
// and fills lookup cache
func rebuildQueue(n *Node) *Queue {
	// rebuild map-cache
	cache := make(map[flow.Identifier]*Node)
	traverse(n, cache)

	return &Queue{
		Head:  n,
		Nodes: cache,
	}
}

// Special case for removing single-childed head element
func dequeue(queue *Queue) *Queue {
	onlyChild := queue.Head.Children[0]

	cache := make(map[flow.Identifier]*Node)

	//copy all but head caches
	for key, val := range queue.Nodes {
		if key != queue.Head.CompleteBlock.Block.ID() {
			cache[key] = val
		}
	}
	return &Queue{
		Head:  onlyChild,
		Nodes: cache,
	}
}

// TryAdd tries to add a new Node to the queue and returns if the operation has been successful
func (q *Queue) TryAdd(completeBlock *execution.CompleteBlock) bool {
	n, ok := q.Nodes[completeBlock.Block.ParentID]
	if !ok {
		return false
	}
	newNode := &Node{
		CompleteBlock: completeBlock,
		Children:      nil,
	}
	n.Children = append(n.Children, newNode)
	q.Nodes[completeBlock.Block.ID()] = newNode
	return true
}

// Attach joins two queues together, fails if new queue head cannot be attached
func (q *Queue) Attach(other *Queue) error {
	n, ok := q.Nodes[other.Head.CompleteBlock.Block.ParentID]
	if !ok {
		return fmt.Errorf("cannot join queues, other queue head does not reference known parent")
	}
	n.Children = append(n.Children, other.Head)
	for identifier, node := range other.Nodes {
		q.Nodes[identifier] = node
	}
	return nil
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
	return q.Head.CompleteBlock, queues
}
