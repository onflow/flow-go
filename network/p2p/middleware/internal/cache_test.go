package internal_test

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/middleware/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewDisallowListCache tests the NewDisallowListCache function. It verifies that the returned disallowListCache
// is not nil.
func TestNewDisallowListCache(t *testing.T) {
	disallowListCache := internal.NewDisallowListCache(uint32(100), unittest.Logger(), metrics.NewNoopCollector())

	// Verify that the new disallowListCache is not nil
	assert.NotNil(t, disallowListCache)
}

// TestDisallowFor_SinglePeer tests the DisallowFor function for a single peer. It verifies that the peerID is
// disallow-listed for the given cause and that the cause is returned when the peerID is disallow-listed again.
func TestDisallowFor_SinglePeer(t *testing.T) {
	disallowListCache := internal.NewDisallowListCache(uint32(100), unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, disallowListCache)

	// disallowing a peerID for a cause when the peerID doesn't exist in the cache
	causes, err := disallowListCache.DisallowFor(peer.ID("peer1"), middleware.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 1)
	require.Contains(t, causes, middleware.DisallowListedCauseAdmin)

	// disallowing a peerID for a cause when the peerID already exists in the cache
	causes, err = disallowListCache.DisallowFor(peer.ID("peer1"), middleware.DisallowListedCauseAlsp)
	require.NoError(t, err)
	require.Len(t, causes, 2)
	require.ElementsMatch(t, causes, []middleware.DisallowListedCause{middleware.DisallowListedCauseAdmin, middleware.DisallowListedCauseAlsp})

	// disallowing a peerID for a duplicate cause
	causes, err = disallowListCache.DisallowFor(peer.ID("peer1"), middleware.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 2)
	require.ElementsMatch(t, causes, []middleware.DisallowListedCause{middleware.DisallowListedCauseAdmin, middleware.DisallowListedCauseAlsp})
}
