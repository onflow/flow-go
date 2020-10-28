package queue

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
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
// Formally, the Queue stores a tree. The height of the tree is the
// number of edges on the longest downward path between the root and any leaf.
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
	headID := queue.Head.Item.ID() // ID computation is about as expensive 1000 Go int additions
	for key, val := range queue.Nodes {
		if key != headID {
			cache[key] = val
		}
	}
	return &Queue{
		Head:    onlyChild,
		Nodes:   cache,
		Highest: queue.Highest,
	}
}

// TryAdd tries to add a new element to the queue.
// A element can only be added if the parent exists in the queue.
// TryAdd(elmt) is an idempotent operation for the same elmt, i.e.
// after the first, subsequent additions of the same elements are NoOps.
// Returns:
// True if and only if _after_ the operation, the element is stored in the
// queue. This is the case if (a) element was newly added to the queue or
// (b) element was already stored in the queue _before_ the call.
// Adding an element fails with return value `false` in the following cases:
//   * element.ParentID() is _not_ stored in the queue
//   * element's height is _unequal to_ its parent's height + 1
func (q *Queue) TryAdd(element Blockify) bool {
	if _, found := q.Nodes[element.ID()]; found {
		// (b) element was already stored in the queue _before_ the call.
		return true
	}
	// at this point, we are sure that the element is _not_ in the queue and therefore,
	// the element cannot be referenced as a child by any other element in the queue
	n, ok := q.Nodes[element.ParentID()]
	if !ok {
		return false
	}
	if n.Item.Height() != element.Height()-1 {
		return false
	}
	newNode := &Node{
		Item:     element,
		Children: nil,
	}
	// we know: element is _not_ (yet) in the queue
	// => it cannot be in _any_ nodes Children list
	// => the following operation is guaranteed to _not_ produce
	//    duplicates in the Children list
	n.Children = append(n.Children, newNode)
	q.Nodes[element.ID()] = newNode
	if element.Height() > q.Highest.Item.Height() {
		q.Highest = newNode
	}
	return true
}

// Attach joins the other queue to this one, modifying this queue in-place.
// Other queue be attached if only if the following conditions hold:
//   * The head of other has a parent in this queue.
//   * Both queues have no nodes in common. Specifically, this means that
//     the head of _other_ cannot be a member of this queue.
// Fails otherwise with an error. CAUTION: failing with an error leaves
// this queue in a _dysfunctional_ (partially joined) state.
func (q *Queue) Attach(other *Queue) error {
	n, ok := q.Nodes[other.Head.Item.ParentID()]
	if !ok {
		return fmt.Errorf("other queue head does not reference known parent")
	}
	n.Children = append(n.Children, other.Head)
	for identifier, node := range other.Nodes {
		if _, ok := q.Nodes[identifier]; ok {
			return fmt.Errorf("queues have common nodes")
		}
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
