package queue

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Node struct {
	Item     Blockify
	Children []*Node
}

// Blockify becuase Blocker seems a bit off.
// Make items behave like a block, so it can be queued
type Blockify interface {
	flow.Entity
	Height() uint64
	ParentID() flow.Identifier
}

// Queue is a fork-aware queue/tree of blocks for use in execution Node, where parallel forks
// can be processed simultaneously. For fast lookup which is predicted to be common case
// all nodes are kept as one queue, which is expected to split into separate queues once
// a fork (multiple children) is reached.
// Note that this is not a thread-safe structure and external synchronisation is required
// to use in concurrent environment
type Queue struct {
	Head    *Node
	Highest *Node
	Nodes   map[flow.Identifier]*Node
}

// Make queue an entity so it can be stored in mempool

func (q *Queue) ID() flow.Identifier {
	return q.Head.Item.ID()
}

func (q *Queue) Checksum() flow.Identifier {
	return q.Head.Item.Checksum()
}

// Size returns number of elements in the queue
func (q *Queue) Size() int {
	return len(q.Nodes)
}

// Returns difference between lowest and highest element in the queue
func (q *Queue) Height() uint64 {
	return q.Highest.Item.Height() - q.Head.Item.Height()
}

// traverse Node children recursively and populate m
func traverse(node *Node, m map[flow.Identifier]*Node, highest *Node) {
	m[node.Item.ID()] = node
	for _, node := range node.Children {
		if node.Item.Height() > highest.Item.Height() {
			*highest = *node
		}
		traverse(node, m, highest)
	}
}

func NewQueue(blockify Blockify) *Queue {
	n := &Node{
		Item:     blockify,
		Children: nil,
	}
	return &Queue{
		Head:    n,
		Highest: n,
		Nodes:   map[flow.Identifier]*Node{n.Item.ID(): n},
	}
}

// rebuildQueue makes a new queue from a Node which was already part of other queue
// and fills lookup cache
func rebuildQueue(n *Node) *Queue {
	// rebuild map-cache
	cache := make(map[flow.Identifier]*Node)
	highest := *n //copy n
	traverse(n, cache, &highest)

	return &Queue{
		Head:    n,
		Nodes:   cache,
		Highest: &highest,
	}
}

// Special case for removing single-childed head element
func dequeue(queue *Queue) *Queue {
	onlyChild := queue.Head.Children[0]

	cache := make(map[flow.Identifier]*Node)

	//copy all but head caches
	for key, val := range queue.Nodes {
		if key != queue.Head.Item.ID() {
			cache[key] = val
		}
	}
	return &Queue{
		Head:    onlyChild,
		Nodes:   cache,
		Highest: queue.Highest,
	}
}

// TryAdd tries to add a new Node to the queue and returns if the operation has been successful
func (q *Queue) TryAdd(executableBlock Blockify) bool {
	n, ok := q.Nodes[executableBlock.ParentID()]
	if !ok {
		return false
	}
	if n.Item.Height() != executableBlock.Height()-1 {
		return false
	}
	newNode := &Node{
		Item:     executableBlock,
		Children: nil,
	}
	n.Children = append(n.Children, newNode)
	q.Nodes[executableBlock.ID()] = newNode
	if executableBlock.Height() > q.Highest.Item.Height() {
		q.Highest = newNode
	}
	return true
}

// Attach joins two queues together, fails if new queue head cannot be attached
func (q *Queue) Attach(other *Queue) error {
	n, ok := q.Nodes[other.Head.Item.ParentID()]
	if !ok {
		return fmt.Errorf("cannot join queues, other queue head does not reference known parent")
	}
	n.Children = append(n.Children, other.Head)
	for identifier, node := range other.Nodes {
		q.Nodes[identifier] = node
	}
	if other.Highest.Item.Height() > q.Highest.Item.Height() {
		q.Highest = other.Highest
	}
	return nil
}

// Dismount removes the head element, returns it and it's children as new queues
func (q *Queue) Dismount() (Blockify, []*Queue) {

	queues := make([]*Queue, len(q.Head.Children))
	if len(q.Head.Children) == 1 { //optimize for most common single-child case
		queues[0] = dequeue(q)
	} else {
		for i, child := range q.Head.Children {
			queues[i] = rebuildQueue(child)
		}
	}
	return q.Head.Item, queues
}
