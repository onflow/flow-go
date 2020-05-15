package node

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"
)

// Node is a struct for constructing our Tree
type Node struct {
	LChild    *Node // Left Child
	RChild    *Node // Right Child
	Height    int   // Height where the Node is at
	Key       []byte
	Value     []byte
	HashValue []byte
}

// newNode creates a new Node with the provided Value and no children
func NewNode(height int) *Node {
	return &Node{
		LChild:    nil,
		RChild:    nil,
		Height:    height,
		Key:       nil,
		Value:     nil,
		HashValue: nil,
	}
}

// PopulateNodeHashValues recursively update nodes with
// the hash Values for intermediate nodes (leafs already has Values after update)
// we only use this function to speed up proof generation,
// for less memory usage we can skip this function
func (n *Node) PopulateNodeHashValues() []byte {
	if n.HashValue != nil {
		return n.HashValue
	}

	// otherwise compute
	h1 := common.GetDefaultHashForHeight(n.Height - 1)
	if n.LChild != nil {
		h1 = n.LChild.PopulateNodeHashValues()
	}
	h2 := common.GetDefaultHashForHeight(n.Height - 1)
	if n.RChild != nil {
		h2 = n.RChild.PopulateNodeHashValues()
	}
	n.HashValue = common.HashInterNode(h1, h2)

	return n.HashValue
}

// GetNodeHash computes the HashValue for the given Node
func (n *Node) GetNodeHash() []byte {
	if n.HashValue != nil {
		return n.HashValue
	}
	return n.ComputeNodeHash(false)
}

// ComputeNodeHash computes the HashValue for the given Node
// if forced it set it won't trust hash Values of children and
// recomputes it.
func (n *Node) ComputeNodeHash(forced bool) []byte {
	// leaf Node (this shouldn't happen)
	if n.LChild == nil && n.RChild == nil {
		if len(n.Value) > 0 {
			return n.GetCompactValue()
		}
		return common.GetDefaultHashForHeight(n.Height)
	}
	// otherwise compute
	h1 := common.GetDefaultHashForHeight(n.Height - 1)
	if n.LChild != nil {
		if forced {
			h1 = n.LChild.ComputeNodeHash(forced)
		} else {
			h1 = n.LChild.GetNodeHash()
		}
	}
	h2 := common.GetDefaultHashForHeight(n.Height - 1)
	if n.RChild != nil {
		if forced {
			h2 = n.RChild.ComputeNodeHash(forced)
		} else {
			h2 = n.RChild.GetNodeHash()
		}
	}
	return common.HashInterNode(h1, h2)
}

// GetCompactValue computes the Value for the Node considering the sub tree to only include this Value and default Values.
func (n *Node) GetCompactValue() []byte {
	return common.ComputeCompactValue(n.Key, n.Value, n.Height)
}

// FmtStr provides formatted string representation of the Node and sub tree
func (n Node) FmtStr(prefix string, path string) string {
	right := ""
	if n.RChild != nil {
		right = fmt.Sprintf("\n%v", n.RChild.FmtStr(prefix+"\t", path+"1"))
	}
	left := ""
	if n.LChild != nil {
		left = fmt.Sprintf("\n%v", n.LChild.FmtStr(prefix+"\t", path+"0"))
	}
	return fmt.Sprintf("%v%v: (k:%v, v:%v, h:%v)[%s] %v %v ", prefix, n.Height, n.Key, hex.EncodeToString(n.Value), hex.EncodeToString(n.HashValue), path, left, right)
}

// DeepCopy returns a deep copy of the Node (including deep copy of children)
func (n *Node) DeepCopy() *Node {
	newNode := &Node{Height: n.Height}

	if n.Value != nil {
		value := make([]byte, len(n.Value))
		copy(value, n.Value)
		newNode.Value = value
	}
	if n.Key != nil {
		key := make([]byte, len(n.Key))
		copy(key, n.Key)
		newNode.Key = key
	}
	if n.LChild != nil {
		newNode.LChild = n.LChild.DeepCopy()
	}
	if n.RChild != nil {
		newNode.RChild = n.RChild.DeepCopy()
	}
	return newNode
}

// Equals compares two nodes and all subsequent children
// this is an expensive call and should only be used
// for limited cases (e.g. testing)
func (n *Node) Equals(o *Node) bool {

	// Height don't match
	if n.Height != o.Height {
		return false
	}

	// Values don't match
	if (n.Value == nil) != (o.Value == nil) {
		return false
	}
	if n.Value != nil && o.Value != nil && !bytes.Equal(n.Value, o.Value) {
		return false
	}

	// keys don't match
	if (n.Key == nil) != (o.Key == nil) {
		return false
	}
	if n.Key != nil && o.Key != nil && !bytes.Equal(n.Key, o.Key) {
		return false
	}

	// hashValues don't match
	if (n.HashValue == nil) != (o.HashValue == nil) {
		return false
	}
	if n.HashValue != nil && o.HashValue != nil && !bytes.Equal(n.HashValue, o.HashValue) {
		return false
	}

	// left children don't match
	if (n.LChild == nil) != (o.LChild == nil) {
		return false
	}
	if n.LChild != nil && o.LChild != nil && !n.LChild.Equals(o.LChild) {
		return false
	}

	// right children don't match
	if (n.RChild == nil) != (o.RChild == nil) {
		return false
	}
	if n.RChild != nil && o.RChild != nil && !n.RChild.Equals(o.RChild) {
		return false
	}

	return true
}
