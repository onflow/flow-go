package internal_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p/node/internal"
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
	causes, err := disallowListCache.DisallowFor(peer.ID("peer1"), network.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 1)
	require.Contains(t, causes, network.DisallowListedCauseAdmin)

	// disallowing a peerID for a cause when the peerID already exists in the cache
	causes, err = disallowListCache.DisallowFor(peer.ID("peer1"), network.DisallowListedCauseAlsp)
	require.NoError(t, err)
	require.Len(t, causes, 2)
	require.ElementsMatch(t, causes, []network.DisallowListedCause{network.DisallowListedCauseAdmin, network.DisallowListedCauseAlsp})

	// disallowing a peerID for a duplicate cause
	causes, err = disallowListCache.DisallowFor(peer.ID("peer1"), network.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 2)
	require.ElementsMatch(t, causes, []network.DisallowListedCause{network.DisallowListedCauseAdmin, network.DisallowListedCauseAlsp})
}

// TestDisallowFor_MultiplePeers tests the DisallowFor function for multiple peers. It verifies that the peerIDs are
// disallow-listed for the given cause and that the cause is returned when the peerIDs are disallow-listed again.
func TestDisallowFor_MultiplePeers(t *testing.T) {
	disallowListCache := internal.NewDisallowListCache(uint32(100), unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, disallowListCache)

	for i := 0; i <= 10; i++ {
		// disallowing a peerID for a cause when the peerID doesn't exist in the cache
		causes, err := disallowListCache.DisallowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
		require.NoError(t, err)
		require.Len(t, causes, 1)
		require.Contains(t, causes, network.DisallowListedCauseAdmin)
	}

	for i := 0; i <= 10; i++ {
		// disallowing a peerID for a cause when the peerID already exists in the cache
		causes, err := disallowListCache.DisallowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAlsp)
		require.NoError(t, err)
		require.Len(t, causes, 2)
		require.ElementsMatch(t, causes, []network.DisallowListedCause{network.DisallowListedCauseAdmin, network.DisallowListedCauseAlsp})
	}

	for i := 0; i <= 10; i++ {
		// getting the disallow-listed causes for a peerID
		causes, disallowListed := disallowListCache.IsDisallowListed(peer.ID(fmt.Sprintf("peer-%d", i)))
		require.True(t, disallowListed)
		require.Len(t, causes, 2)
		require.ElementsMatch(t, causes, []network.DisallowListedCause{network.DisallowListedCauseAdmin, network.DisallowListedCauseAlsp})
	}
}

// TestAllowFor_SinglePeer is a unit test function to verify the behavior of DisallowListCache for a single peer.
// The test checks the following functionalities in sequence:
// 1. Allowing a peerID for a cause when the peerID already exists in the cache.
// 2. Disallowing the peerID for a cause when the peerID doesn't exist in the cache.
// 3. Getting the disallow-listed causes for the peerID.
// 4. Allowing a peerID for a cause when the peerID already exists in the cache.
// 5. Getting the disallow-listed causes for the peerID.
// 6. Disallowing the peerID for a cause.
// 7. Allowing the peerID for a different cause than it is disallowed when the peerID already exists in the cache.
// 8. Disallowing the peerID for another cause.
// 9. Allowing the peerID for the first cause.
// 10. Allowing the peerID for the second cause.
func TestAllowFor_SinglePeer(t *testing.T) {
	disallowListCache := internal.NewDisallowListCache(uint32(100), unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, disallowListCache)
	peerID := peer.ID("peer1")

	// allowing the peerID for a cause when the peerID already exists in the cache
	causes := disallowListCache.AllowFor(peerID, network.DisallowListedCauseAdmin)
	require.Len(t, causes, 0)

	// disallowing the peerID for a cause when the peerID doesn't exist in the cache
	causes, err := disallowListCache.DisallowFor(peerID, network.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 1)
	require.Contains(t, causes, network.DisallowListedCauseAdmin)

	// getting the disallow-listed causes for the peerID
	causes, disallowListed := disallowListCache.IsDisallowListed(peerID)
	require.True(t, disallowListed)
	require.Len(t, causes, 1)
	require.Contains(t, causes, network.DisallowListedCauseAdmin)

	// allowing a peerID for a cause when the peerID already exists in the cache
	causes = disallowListCache.AllowFor(peerID, network.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 0)

	// getting the disallow-listed causes for the peerID
	causes, disallowListed = disallowListCache.IsDisallowListed(peerID)
	require.False(t, disallowListed)
	require.Len(t, causes, 0)

	// disallowing the peerID for a cause
	causes, err = disallowListCache.DisallowFor(peerID, network.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 1)

	// allowing the peerID for a different cause than it is disallowed when the peerID already exists in the cache
	causes = disallowListCache.AllowFor(peerID, network.DisallowListedCauseAlsp)
	require.NoError(t, err)
	require.Len(t, causes, 1)
	require.Contains(t, causes, network.DisallowListedCauseAdmin) // the peerID is still disallow-listed for the previous cause

	// disallowing the peerID for another cause
	causes, err = disallowListCache.DisallowFor(peerID, network.DisallowListedCauseAlsp)
	require.NoError(t, err)
	require.Len(t, causes, 2)
	require.ElementsMatch(t, causes, []network.DisallowListedCause{network.DisallowListedCauseAdmin, network.DisallowListedCauseAlsp})

	// allowing the peerID for the first cause
	causes = disallowListCache.AllowFor(peerID, network.DisallowListedCauseAdmin)
	require.NoError(t, err)
	require.Len(t, causes, 1)
	require.Contains(t, causes, network.DisallowListedCauseAlsp) // the peerID is still disallow-listed for the previous cause

	// allowing the peerID for the second cause
	causes = disallowListCache.AllowFor(peerID, network.DisallowListedCauseAlsp)
	require.NoError(t, err)
	require.Len(t, causes, 0)
}

// TestAllowFor_MultiplePeers_Sequentially is a unit test function to test the behavior of DisallowListCache with multiple peers.
// The test checks the following functionalities in sequence:
// 1. Allowing a peerID for a cause when the peerID doesn't exist in the cache.
// 2. Disallowing peers for a cause.
// 3. Getting the disallow-listed causes for a peerID.
// 4. Allowing the peer ids for a cause different than the one they are disallow-listed for.
// 5. Disallowing the peer ids for a different cause.
// 6. Allowing the peer ids for the first cause.
// 7. Allowing the peer ids for the second cause.
func TestAllowFor_MultiplePeers_Sequentially(t *testing.T) {
	disallowListCache := internal.NewDisallowListCache(uint32(100), unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, disallowListCache)

	for i := 0; i <= 10; i++ {
		// allowing a peerID for a cause when the peerID doesn't exist in the cache
		causes := disallowListCache.AllowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
		require.Len(t, causes, 0)
	}

	for i := 0; i <= 10; i++ {
		// disallowing peers for a cause
		causes, err := disallowListCache.DisallowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAlsp)
		require.NoError(t, err)
		require.Len(t, causes, 1)
		require.Contains(t, causes, network.DisallowListedCauseAlsp)
	}

	for i := 0; i <= 10; i++ {
		// getting the disallow-listed causes for a peerID
		causes, disallowListed := disallowListCache.IsDisallowListed(peer.ID(fmt.Sprintf("peer-%d", i)))
		require.True(t, disallowListed)
		require.Len(t, causes, 1)
		require.Contains(t, causes, network.DisallowListedCauseAlsp)
	}

	for i := 0; i <= 10; i++ {
		// allowing the peer ids for a cause different than the one they are disallow-listed for
		causes := disallowListCache.AllowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
		require.Len(t, causes, 1)
		require.Contains(t, causes, network.DisallowListedCauseAlsp)
	}

	for i := 0; i <= 10; i++ {
		// disallowing the peer ids for a different cause
		causes, err := disallowListCache.DisallowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
		require.NoError(t, err)
		require.Len(t, causes, 2)
		require.ElementsMatch(t, causes, []network.DisallowListedCause{network.DisallowListedCauseAdmin, network.DisallowListedCauseAlsp})
	}

	for i := 0; i <= 10; i++ {
		// allowing the peer ids for the first cause
		causes := disallowListCache.AllowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
		require.Len(t, causes, 1)
		require.Contains(t, causes, network.DisallowListedCauseAlsp)
	}

	for i := 0; i <= 10; i++ {
		// allowing the peer ids for the second cause
		causes := disallowListCache.AllowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAlsp)
		require.Len(t, causes, 0)
	}
}

// TestAllowFor_MultiplePeers_Concurrently is a unit test function that verifies the behavior of DisallowListCache
// when multiple peerIDs are added and managed concurrently. This test is designed to confirm that DisallowListCache
// works as expected under concurrent access, an important aspect for a system dealing with multiple connections.
//
// The test runs multiple goroutines simultaneously, each handling a different peerID and performs the following
// operations in the sequence:
// 1. Allowing a peerID for a cause when the peerID doesn't exist in the cache.
// 2. Disallowing peers for a cause.
// 3. Getting the disallow-listed causes for a peerID.
// 4. Allowing the peer ids for a cause different than the one they are disallow-listed for.
// 5. Disallowing the peer ids for a different cause.
// 6. Allowing the peer ids for the first cause.
// 7. Allowing the peer ids for the second cause.
// 8. Getting the disallow-listed causes for a peerID.
// 9. Allowing a peerID for a cause when the peerID doesn't exist in the cache for a new set of peers.
func TestAllowFor_MultiplePeers_Concurrently(t *testing.T) {
	disallowListCache := internal.NewDisallowListCache(uint32(100), unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, disallowListCache)

	var wg sync.WaitGroup
	for i := 0; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// allowing a peerID for a cause when the peerID doesn't exist in the cache
			causes := disallowListCache.AllowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
			require.Len(t, causes, 0)
		}(i)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	for i := 0; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// disallowing peers for a cause
			causes, err := disallowListCache.DisallowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAlsp)
			require.NoError(t, err)
			require.Len(t, causes, 1)
			require.Contains(t, causes, network.DisallowListedCauseAlsp)
		}(i)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	for i := 0; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// getting the disallow-listed causes for a peerID
			causes, disallowListed := disallowListCache.IsDisallowListed(peer.ID(fmt.Sprintf("peer-%d", i)))
			require.Len(t, causes, 1)
			require.True(t, disallowListed)
			require.Contains(t, causes, network.DisallowListedCauseAlsp)
		}(i)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	for i := 0; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// allowing the peer ids for a cause different than the one they are disallow-listed for
			causes := disallowListCache.AllowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
			require.Len(t, causes, 1)
			require.Contains(t, causes, network.DisallowListedCauseAlsp)
		}(i)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	for i := 0; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// disallowing the peer ids for a different cause
			causes, err := disallowListCache.DisallowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
			require.NoError(t, err)
			require.Len(t, causes, 2)
			require.ElementsMatch(t, causes, []network.DisallowListedCause{network.DisallowListedCauseAdmin, network.DisallowListedCauseAlsp})
		}(i)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	for i := 0; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// allowing the peer ids for the first cause
			causes := disallowListCache.AllowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
			require.Len(t, causes, 1)
			require.Contains(t, causes, network.DisallowListedCauseAlsp)
		}(i)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	for i := 0; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// allowing the peer ids for the second cause
			causes := disallowListCache.AllowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAlsp)
			require.Len(t, causes, 0)
		}(i)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	for i := 0; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// getting the disallow-listed causes for a peerID
			causes, disallowListed := disallowListCache.IsDisallowListed(peer.ID(fmt.Sprintf("peer-%d", i)))
			require.False(t, disallowListed)
			require.Len(t, causes, 0)
		}(i)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	for i := 11; i <= 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// allowing a peerID for a cause when the peerID doesn't exist in the cache
			causes := disallowListCache.AllowFor(peer.ID(fmt.Sprintf("peer-%d", i)), network.DisallowListedCauseAdmin)
			require.Len(t, causes, 0)
		}(i)
	}
}
