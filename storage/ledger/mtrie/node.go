package mtrie

import (
	"encoding/hex"
	"fmt"
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
	n := new(node)
	n.height = height
	n.lChild = nil
	n.rChild = nil
	n.key = nil
	n.value = nil
	n.hashValue = nil
	return n
}

// GetValue returns the value of the node.
func (n *node) GetValue() []byte {
	return n.value
}

// GetHeight returns the height of the node.
func (n *node) GetHeight() int {
	return n.height
}

// FmtStr provides formated string represntation of the node and sub tree
func (n node) FmtStr(prefix string, path string) string {
	right := ""
	if n.rChild != nil {
		right = fmt.Sprintf("\n%v", n.rChild.FmtStr(prefix+"\t", path+"1"))
	}
	left := ""
	if n.lChild != nil {
		left = fmt.Sprintf("\n%v", n.lChild.FmtStr(prefix+"\t", path+"0"))
	}
	return fmt.Sprintf("%v%v: (%v,%v)[%s] %v %v ", prefix, n.height, n.key, hex.EncodeToString(n.value), path, left, right)
}

// return a deep copy of the node (including deep copy of children)
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
