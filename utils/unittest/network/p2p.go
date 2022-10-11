package network

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// StopNodes stop all nodes in the input slice
func StopNodes(t *testing.T, nodes []*p2pnode.Node) {
	for _, n := range nodes {
		StopNode(t, n)
	}
}

// StopNode stops node
func StopNode(t *testing.T, node *p2pnode.Node) {
	err := node.Stop()
	assert.NoError(t, err)
}
