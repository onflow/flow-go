// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package merkle

import (
	"github.com/jrick/bitset"
	"golang.org/x/crypto/blake2b"
)

// Tree represents a binary patricia merkle tree. The difference with a normal
// merkle tree is that it compresses paths that lead to a single leaf into a
// single intermediary node, which makes it significantly more space-efficient
// and a lot harder to exploit for denial-of-service attacks. On the downside,
// it makes insertions and deletions more complex, as we need to split nodes
// and merge them, depending on whethere their are leaves or not.
type Tree struct {
	root node
}

// NewTree creates a new empty patricia merkle tree.
func NewTree() *Tree {
	t := &Tree{}
	return t
}

// Put will stores the given value in the trie under the given key. If the key
// already exists, it will replace the value and return true.
func (t *Tree) Put(key []byte, val interface{}) bool {

	// the path through the tree is determined by the key; we decide whether to
	// go left or right based on whether the next bit is set or not
	path := bitset.Bytes(key[:])

	// we use a pointer that points at the current node in the tree
	cur := &t.root

	// we use an index to keep track of the bit we are currently looking at
	index := uint(0)

	// the for statement keeps running until we reach a leaf in the merkle tree
	// if the leaf is nil, it was empty and we insert a new value
	// if the leaf is a valid pointer, we overwrite the previous value
PutLoop:
	for {
		switch n := (*cur).(type) {

		// if we have a full node, we have a node on each side to go to, so we
		// just pick the next node based on whether the bit is set or not
		case *full:

			// if the bit is 0 (false), we go left
			// otherwise, it's 1 (true) and we go right
			if !path.Get(int(index)) {
				cur = &n.left
			} else {
				cur = &n.right
			}

			// we forward the index by one to look at the next bit
			index++

			continue PutLoop

		// if we have a short node, we have a path of several bits to the next
		// node; in that case, we use as much of the shared path as possible
		case *short:

			// first, we find out how many bits we have in common
			nodePath := bitset.Bytes(n.path)
			var commonCount uint
			for i := uint(0); i < n.count; i++ {
				if path.Get(int(i+index)) != nodePath.Get(int(i)) {
					break
				}
				commonCount++
			}

			// if the common and node count are equal, we share all of the path
			// we can simply forward to the child of the short node and continue
			if commonCount == n.count {
				cur = &n.child
				index += commonCount
				continue PutLoop
			}

			// if the common count is non-zero, we share some of the path;
			// first, we insert a common short node for the shared path
			if commonCount > 0 {
				commonPath := bitset.NewBytes(int(commonCount))
				for i := uint(0); i < commonCount; i++ {
					commonPath.SetBool(int(i), path.Get(int(i+index)))
				}
				commonNode := &short{count: commonCount, path: []byte(commonPath)}
				*cur = commonNode
				cur = &commonNode.child
				index = index + commonCount
			}

			// we then insert a full node that splits the tree after the shared
			// path; we set our pointer to the side that lies on our path,
			// and use a remaining pointer for the other side of the node
			var remain *node
			splitNode := &full{}
			*cur = splitNode
			if nodePath.Get(int(commonCount)) {
				cur = &splitNode.left
				remain = &splitNode.right
			} else {
				cur = &splitNode.right
				remain = &splitNode.left
			}
			index++

			// we can continue our insertion at this point, but we should first
			// insert the correct node on the other side of the created full
			// node; if we have remaining path, we create a short node and
			// forward to its path; finally, we set the leaf to original leaf
			remainCount := n.count - commonCount - 1
			if remainCount > 0 {
				remainPath := bitset.NewBytes(int(remainCount))
				for i := uint(0); i < remainCount; i++ {
					remainPath.SetBool(int(i), nodePath.Get(int(i+commonCount+1)))
				}
				remainNode := &short{count: remainCount, path: []byte(remainPath)}
				*remain = remainNode
				remain = &remainNode.child
			}
			*remain = n.child

			continue PutLoop

		// if we have a leaf node, we reached a non-empty leaf
		case *leaf:

			// replace the current value with the new one
			n.val = val

			// return true to indicate that we overwrote
			return true

		// if we have nil, we reached the end of any shared path
		case nil:

			// if we have reached the end of the key, insert the new value
			totalCount := uint(len(key)) * 8
			if index == totalCount {
				// Make a copy of the key, to ensure we have the only pointer to it
				keyCopy := make([]byte, len(key))
				copy(keyCopy, key)
				*cur = &leaf{
					key: keyCopy,
					val: val,
				}
				return false
			}

			// otherwise, insert a short node with the remainder of the path
			finalCount := totalCount - index
			finalPath := bitset.NewBytes(int(finalCount))
			for i := uint(0); i < finalCount; i++ {
				finalPath.SetBool(int(i), path.Get(int(index+i)))
			}
			finalNode := &short{count: finalCount, path: []byte(finalPath)}
			*cur = finalNode
			cur = &finalNode.child
			index += finalCount

			continue PutLoop
		}
	}
}

// Get will retrieve the value associated with the given key. It returns true
// if the key was found and false otherwise.
func (t *Tree) Get(key []byte) (interface{}, bool) {

	// we start at the root again
	cur := &t.root

	// we use the given key as path again
	path := bitset.Bytes(key[:])

	// and we start at a zero index in the path
	index := uint(0)

GetLoop:
	for {
		switch n := (*cur).(type) {

		// if we have a full node, we can follow the path for at least one more
		// bit, so go left or right depending on whether it's set or not
		case *full:

			// forward pointer and index to the correct child
			if !path.Get(int(index)) {
				cur = &n.left
			} else {
				cur = &n.right
			}
			index++

			continue GetLoop

		// if we have a short path, we can only follow the path if we have all
		// of the short node path in common with the key
		case *short:

			// if any part of the path doesn't match, key doesn't exist
			nodePath := bitset.Bytes(n.path)
			for i := uint(0); i < n.count; i++ {
				if path.Get(int(i+index)) != nodePath.Get(int(i)) {
					return nil, false
				}
			}

			// forward pointer and index to child
			cur = &n.child
			index += n.count

			continue GetLoop

		// if we have a leaf, we found the key, return value and true
		case *leaf:

			return n.val, true

		// if we have a nil node, key doesn't exist, return nil and false
		case nil:

			return nil, false
		}
	}
}

// Del will remove the value associated with the given key from the patricia
// merkle trie. It will return true if they key was found and false otherwise.
// Internally, any parent nodes between the leaf up to the closest shared path
// will be deleted or merged, which keeps the trie deterministic regardless of
// insertion and deletion orders.
func (t *Tree) Del(key []byte) bool {

	// we initialize three pointers pointing to a dummy empty node
	// this is used to keep track of the node we last pointed to, as well as
	// its parent and grand parent, which is needed in case we remove a full
	// node and have to merge several other nodes into a short node; otherwise,
	// we would not keep the tree as compact as possible, and it would no longer
	// be deterministic after deletes
	dummy := node(&struct{}{})
	last, parent, grand := &dummy, &dummy, &dummy

	// we set the current pointer to the root
	cur := &t.root

	// we use the key as the path
	path := bitset.Bytes(key[:])

	// the index points to the bit we are processing in the path
	index := uint(0)

DelLoop:
	for {
		switch n := (*cur).(type) {

		// if we have a full node, we forward all of the pointers
		case *full:

			// keep track of grand-parent, parent and node for cleanup
			grand = parent
			parent = last
			last = cur

			// forward pointer and index to the correct child
			if !path.Get(int(index)) {
				cur = &n.left
			} else {
				cur = &n.right
			}

			index++

			continue DelLoop

		// if we have a short node, we forward by all of the common path if
		// possible; otherwise the node wasn't found
		case *short:

			// keep track of grand-parent, parent and node for cleanup
			grand = parent
			parent = last
			last = cur

			// if the path doesn't match at any point, we can't find the node
			nodePath := bitset.Bytes(n.path)
			for i := uint(0); i < n.count; i++ {
				if path.Get(int(i+index)) != nodePath.Get(int(i)) {
					return false
				}
			}

			// forward pointer and index to the node child
			cur = &n.child
			index += n.count

			continue DelLoop

		// if we have a leaf node, we remove it and continue with cleanup
		case *leaf:

			// replace the current pointer with nil to delete the node
			*cur = nil

			break DelLoop

		// if we reach nil, the node doesn't exist
		case nil:

			return false
		}
	}

	// if the last node before reaching the leaf is a short node, we set it to
	// nil to remove it from the tree and move the pointer to its parent
	_, ok := (*last).(*short)
	if ok {
		*last = nil
		last = parent
		parent = grand
	}

	// if the last node here is not a full node, we are done; we never have two
	// short nodes in a row, which means we have reached the root
	f, ok := (*last).(*full)
	if !ok {
		return true
	}

	// if the last node is a full node, we need to convert it into a short node
	// that holds the undeleted child and the corresponding bit as path
	var n *short
	newPath := bitset.NewBytes(1)
	if f.left != nil {
		newPath.SetBool(0, false)
		n = &short{count: 1, path: newPath, child: f.left}
	} else {
		newPath.SetBool(0, true)
		n = &short{count: 1, path: newPath, child: f.right}
	}
	*last = n

	// if the child is also a short node, we have to merge them and use the
	// child's child as the child of the merged short node
	c, ok := n.child.(*short)
	if ok {
		t.merge(n, c)
	}

	// if the parent is also a short node, we have to merge them and use the
	// current child as the child of the merged node
	p, ok := (*parent).(*short)
	if ok {
		t.merge(p, n)
	}

	// NOTE: if neither the parent nor the child are short nodes, we simply
	// bypass both conditional scopes and land here right away

	return true
}

// Hash will return the root hash of this patricia merkle tree.
func (t *Tree) Hash() []byte {
	hash := t.nodeHash(t.root)
	return hash
}

// merge will merge a child short node into a parent short node.
func (t *Tree) merge(p *short, c *short) {
	totalCount := p.count + c.count
	totalPath := bitset.NewBytes(int(totalCount))
	parentPath := bitset.Bytes(p.path)
	for i := uint(0); i < p.count; i++ {
		totalPath.SetBool(int(i), parentPath.Get(int(i)))
	}
	childPath := bitset.Bytes(c.path)
	for i := uint(0); i < c.count; i++ {
		totalPath.SetBool(int(i+p.count), childPath.Get(int(i)))
	}
	p.count = totalCount
	p.path = []byte(totalPath)
	p.child = c.child
}

// nodeHash will return the hash of a given node.
func (t *Tree) nodeHash(node node) []byte {

	// use the blake2b hash for now
	h, _ := blake2b.New256(nil)

	switch n := node.(type) {

	// for the full node, we concatenate & hash the right hash and the left hash
	case *full:
		_, _ = h.Write(t.nodeHash(n.left))
		_, _ = h.Write(t.nodeHash(n.right))
		return h.Sum(nil)

	// for the short node, we concatenate & hash the count, path and child hash
	case *short:
		_, _ = h.Write([]byte{byte(n.count)})
		_, _ = h.Write(n.path)
		_, _ = h.Write(t.nodeHash(n.child))
		return h.Sum(nil)

	// for leafs, we simply use the key as the hash
	case *leaf:
		return n.key[:]

		// for a nil node (empty root) we use the zero hash
	case nil:
		return h.Sum(nil)

	// this should never happen
	default:
		panic("invalid node type")
	}
}
