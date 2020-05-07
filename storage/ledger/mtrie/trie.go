package mtrie

import (
	"sync"
)

// MTrie is a fully in memory trie with option to persist to disk
type MTrie struct {
	root           *node
	parent         *MTrie
	values         map[string][]byte
	rootHash       []byte
	parentRootHash []byte
	lock           sync.Mutex
}

// SetParent sets parents for this trie
func (mt *MTrie) SetParent(pt *MTrie) {
	mt.parent = pt
	mt.parentRootHash = pt.rootHash
}

// NewMTrie returns the same root
func NewMTrie(root *node) *MTrie {
	return &MTrie{root: root, values: make(map[string][]byte)}
}
