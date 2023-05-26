package internal_test

import (
	"fmt"
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

// TestDisallowFor_MultiplePeers tests the DisallowFor function for multiple peers. It verifies that the peerIDs are
// disallow-listed for the given cause and that the cause is returned when the peerIDs are disallow-listed again.
func TestDisallowFor_MultiplePeers(t *testing.T) {
	disallowListCache := internal.NewDisallowListCache(uint32(100), unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, disallowListCache)

	for i := 0; i <= 10; i++ {
		// disallowing a peerID for a cause when the peerID doesn't exist in the cache
		causes, err := disallowListCache.DisallowFor(peer.ID(fmt.Sprintf("peer-%d", i)), middleware.DisallowListedCauseAdmin)
		require.NoError(t, err)
		require.Len(t, causes, 1)
		require.Contains(t, causes, middleware.DisallowListedCauseAdmin)
	}

	for i := 0; i <= 10; i++ {
		// disallowing a peerID for a cause when the peerID already exists in the cache
		causes, err := disallowListCache.DisallowFor(peer.ID(fmt.Sprintf("peer-%d", i)), middleware.DisallowListedCauseAlsp)
		require.NoError(t, err)
		require.Len(t, causes, 2)
		require.ElementsMatch(t, causes, []middleware.DisallowListedCause{middleware.DisallowListedCauseAdmin, middleware.DisallowListedCauseAlsp})
	}

	for i := 0; i <= 10; i++ {
		// getting the disallow-listed causes for a peerID
		causes := disallowListCache.GetAllDisallowedListCausesFor(peer.ID(fmt.Sprintf("peer-%d", i)))
		require.Len(t, causes, 2)
		require.ElementsMatch(t, causes, []middleware.DisallowListedCause{middleware.DisallowListedCauseAdmin, middleware.DisallowListedCauseAlsp})
	}
}

// TestAllowFor_SinglePeer tests the AllowFor function for a single peer.
// The test verifies the behavior of cache for:
// 1. Allowing a peerID for a cause when it is not disallow-listed.
// 2. Allowing a peerID for a cause when it is disallow-listed for the same cause.
// 3. Disallowing a peerID for a cause when it is disallow-listed for a different cause.
// 4. Disallowing a peerID for a cause when it is not disallow-listed.
func TestAllowFor_SinglePeer(t *testing.T) {
	disallowListCache := internal.NewDisallowListCache(uint32(100), unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, disallowListCache)
	peerID := peer.ID("peer1")

	// allowing the peerID for a cause when the peerID already exists in the cache
	causes := disallowListCache.AllowFor(peerID, middleware.DisallowListedCauseAdmin)
	require.Len(t, causes, 0)

	// disallowing the peerID for a cause when the peerID doesn't exist in the cache
	causes, err := disallowListCache.DisallowFor(peerID, middleware.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 1)
	require.Contains(t, causes, middleware.DisallowListedCauseAdmin)

	// getting the disallow-listed causes for the peerID
	causes = disallowListCache.GetAllDisallowedListCausesFor(peerID)
	require.Len(t, causes, 1)
	require.Contains(t, causes, middleware.DisallowListedCauseAdmin)

	// allowing a peerID for a cause when the peerID already exists in the cache
	causes = disallowListCache.AllowFor(peerID, middleware.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 0)

	// getting the disallow-listed causes for the peerID
	causes = disallowListCache.GetAllDisallowedListCausesFor(peerID)
	require.Len(t, causes, 0)

	// disallowing the peerID for a cause
	causes, err = disallowListCache.DisallowFor(peerID, middleware.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 1)

	// allowing the peerID for a different cause than it is disallowed when the peerID already exists in the cache
	causes = disallowListCache.AllowFor(peerID, middleware.DisallowListedCauseAlsp)
	require.NoError(t, err)
	require.Len(t, causes, 1)
	require.Contains(t, causes, middleware.DisallowListedCauseAdmin) // the peerID is still disallow-listed for the previous cause

	// disallowing the peerID for another cause
	causes, err = disallowListCache.DisallowFor(peerID, middleware.DisallowListedCauseAlsp)
	require.NoError(t, err)
	require.Len(t, causes, 2)
	require.ElementsMatch(t, causes, []middleware.DisallowListedCause{middleware.DisallowListedCauseAdmin, middleware.DisallowListedCauseAlsp})

	// allowing the peerID for the first cause
	causes = disallowListCache.AllowFor(peerID, middleware.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 1)
	require.Contains(t, causes, middleware.DisallowListedCauseAlsp) // the peerID is still disallow-listed for the previous cause

	// allowing the peerID for the second cause
	causes = disallowListCache.AllowFor(peerID, middleware.DisallowListedCauseAlsp)
	require.NoError(t, err)
	require.Len(t, causes, 0)
}
