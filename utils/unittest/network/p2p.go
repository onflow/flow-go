package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

// StopNodes stop all nodes in the input slice
func StopNodes(t *testing.T, nodes []*p2p.Node) {
	for _, n := range nodes {
		StopNode(t, n)
	}
}

// StopNode stops node
func StopNode(t *testing.T, node *p2p.Node) {
	done, err := node.Stop()
	assert.NoError(t, err)
	unittest.RequireCloseBefore(t, done, 1*time.Second, "could not stop node on ime")
}
