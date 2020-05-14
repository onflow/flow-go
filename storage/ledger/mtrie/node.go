package mtrie

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"
)

// node is a struct for constructing our Tree
type node struct {
	lChild    *node // Left Child
	rChild    *node // Right Child
	height    int   // Height where the node is at
	key       []byte
	value     []byte
	hashValue []byte
}

// newNode creates a new node with the provided value and no children
func newNode(height int) *node {
	return &node{
		lChild:    nil,
		rChild:    nil,
		height:    height,
		key:       nil,
		value:     nil,
		hashValue: nil,
	}
}

// PopulateNodeHashValues recursively update nodes with
// the hash values for intermediate nodes (leafs already has values after update)
// we only use this function to speed up proof generation,
// for less memory usage we can skip this function
func (n *node) PopulateNodeHashValues() []byte {
	if n.hashValue != nil {
		return n.hashValue
	}

	// otherwise compute
	h1 := common.GetDefaultHashForHeight(n.height - 1)
	if n.lChild != nil {
		h1 = n.lChild.PopulateNodeHashValues()
	}
	h2 := common.GetDefaultHashForHeight(n.height - 1)
	if n.rChild != nil {
		h2 = n.rChild.PopulateNodeHashValues()
	}
	n.hashValue = common.HashInterNode(h1, h2)

	return n.hashValue
}

// GetNodeHash computes the hashValue for the given node
func (n *node) GetNodeHash() []byte {
	if n.hashValue != nil {
		return n.hashValue
	}
	return n.ComputeNodeHash(false)
}

// ComputeNodeHash computes the hashValue for the given node
// if forced it set it won't trust hash values of children and
// recomputes it.
func (n *node) ComputeNodeHash(forced bool) []byte {
	// leaf node (this shouldn't happen)
	if n.lChild == nil && n.rChild == nil {
		if len(n.value) > 0 {
			return n.GetCompactValue()
		}
		return common.GetDefaultHashForHeight(n.height)
	}
	// otherwise compute
	h1 := common.GetDefaultHashForHeight(n.height - 1)
	if n.lChild != nil {
		if forced {
			h1 = n.lChild.ComputeNodeHash(forced)
		} else {
			h1 = n.lChild.GetNodeHash()
		}
	}
	h2 := common.GetDefaultHashForHeight(n.height - 1)
	if n.rChild != nil {
		if forced {
			h2 = n.rChild.ComputeNodeHash(forced)
		} else {
			h2 = n.rChild.GetNodeHash()
		}
	}
	return common.HashInterNode(h1, h2)
}

// GetCompactValue computes the value for the node considering the sub tree to only include this value and default values.
func (n *node) GetCompactValue() []byte {
	return common.ComputeCompactValue(n.key, n.value, n.height)
}

// FmtStr provides formatted string representation of the node and sub tree
func (n node) FmtStr(prefix string, path string) string {
	right := ""
	if n.rChild != nil {
		right = fmt.Sprintf("\n%v", n.rChild.FmtStr(prefix+"\t", path+"1"))
	}
	left := ""
	if n.lChild != nil {
		left = fmt.Sprintf("\n%v", n.lChild.FmtStr(prefix+"\t", path+"0"))
	}
	return fmt.Sprintf("%v%v: (k:%v, v:%v, h:%v)[%s] %v %v ", prefix, n.height, n.key, hex.EncodeToString(n.value), hex.EncodeToString(n.hashValue), path, left, right)
}

// DeepCopy returns a deep copy of the node (including deep copy of children)
func (n *node) DeepCopy() *node {
	newNode := &node{height: n.height}

	if n.value != nil {
		value := make([]byte, len(n.value))
		copy(value, n.value)
		newNode.value = value
	}
	if n.key != nil {
		key := make([]byte, len(n.key))
		copy(key, n.key)
		newNode.key = key
	}
	if n.lChild != nil {
		newNode.lChild = n.lChild.DeepCopy()
	}
	if n.rChild != nil {
		newNode.rChild = n.rChild.DeepCopy()
	}
	return newNode
}

// Equals compares two nodes and all subsequent children
// this is an expensive call and should only be used
// for limited cases (e.g. testing)
func (n *node) Equals(o *node) bool {

	// height don't match
	if n.height != o.height {
		return false
	}

	// values don't match
	if (n.value == nil) != (o.value == nil) {
		return false
	}
	if n.value != nil && o.value != nil && !bytes.Equal(n.value, o.value) {
		return false
	}

	// keys don't match
	if (n.key == nil) != (o.key == nil) {
		return false
	}
	if n.key != nil && o.key != nil && !bytes.Equal(n.key, o.key) {
		return false
	}

	// hashValues don't match
	if (n.hashValue == nil) != (o.hashValue == nil) {
		return false
	}
	if n.hashValue != nil && o.hashValue != nil && !bytes.Equal(n.hashValue, o.hashValue) {
		return false
	}

	// left children don't match
	if (n.lChild == nil) != (o.lChild == nil) {
		return false
	}
	if n.lChild != nil && o.lChild != nil && !n.lChild.Equals(o.lChild) {
		return false
	}

	// right children don't match
	if (n.rChild == nil) != (o.rChild == nil) {
		return false
	}
	if n.rChild != nil && o.rChild != nil && !n.rChild.Equals(o.rChild) {
		return false
	}

	return true
}
