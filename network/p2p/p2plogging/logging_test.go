package p2plogging_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/p2p/p2plogging"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
)

// TestPeerIdLogging checks the end-to-end functionality of the PeerId logger helper.
// It ensures that the PeerId logger helper returns the same string as the peer.ID.String() method.
func TestPeerIdLogging(t *testing.T) {
	pid := p2ptest.PeerIdFixture(t)
	pidStr := p2plogging.PeerId(pid)
	require.Equal(t, pid.String(), pidStr)
}
