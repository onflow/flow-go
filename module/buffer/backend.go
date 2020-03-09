package buffer

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// item represents an item in the cache: a block header, payload, and the ID
// of the node that sent it to us. The payload is generic.
type item struct {
	originID flow.Identifier
	header   *flow.Header
	payload  interface{}
}

// backend implements a simple cache of pending blocks, indexed by parent ID.
type backend struct {
	// map of pending blocks, keyed by parent ID
	byParent map[flow.Identifier][]*item
	// set of pending blocks, keyed by ID to avoid duplication
	dedup map[flow.Identifier]struct{}
}

// newBackend returns a new pending block cache.
func newBackend() *backend {
	cache := &backend{
		byParent: make(map[flow.Identifier][]*item),
		dedup:    make(map[flow.Identifier]struct{}),
	}
	return cache
}

// add adds the item to the cache, returning false if it already exists and
// true otherwise.
func (c *backend) add(originID flow.Identifier, header *flow.Header, payload interface{}) bool {
	_, exists := c.dedup[header.ID()]
	if exists {
		return false
	}

	item := &item{
		header:   header,
		originID: originID,
	}

	c.byParent[header.ParentID] = append(c.byParent[header.ParentID], item)
	c.dedup[header.ID()] = struct{}{}

	return true
}

// byParentID returns a list of cached blocks with the given parent. If no such
// blocks exist, returns false.
func (c *backend) byParentID(parentID flow.Identifier) ([]*item, bool) {
	forParent, exists := c.byParent[parentID]
	if !exists {
		return nil, false
	}

	return forParent, true
}

// dropForParent removes all cached blocks with the given parent (non-recursively).
func (c *backend) dropForParent(parentID flow.Identifier) {
	children, exists := c.byParent[parentID]
	if !exists {
		return
	}

	for _, child := range children {
		delete(c.dedup, child.header.ID())
	}
	delete(c.byParent, parentID)
}
