package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/network/p2p/p2plogging/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewPeerIdCache tests the basic functionality of the peer ID cache. It ensures that the cache
// is created successfully.
func TestNewPeerIdCache(t *testing.T) {
	cacheSize := 100
	cache, err := internal.NewPeerIdCache(cacheSize)
	assert.NoError(t, err)
	assert.NotNil(t, cache)
}

// TestPeerIdCache_PeerIdString tests the basic functionality of the peer ID cache. It ensures that the cache
// returns the same string as the peer.ID.String() method.
func TestPeerIdCache_PeerIdString(t *testing.T) {
	cacheSize := 100
	cache, err := internal.NewPeerIdCache(cacheSize)
	assert.NoError(t, err)

	t.Run("existing peer ID", func(t *testing.T) {
		pid := unittest.PeerIdFixture(t)
		pidStr := cache.PeerIdString(pid)
		assert.NotEmpty(t, pidStr)
		assert.Equal(t, pid.String(), pidStr)

		gotPidStr, ok := cache.ByPeerId(pid)
		assert.True(t, ok, "expected pid to be in the cache")
		assert.Equal(t, pid.String(), gotPidStr)
	})

	t.Run("non-existing peer ID", func(t *testing.T) {
		pid1 := unittest.PeerIdFixture(t)
		pid2 := unittest.PeerIdFixture(t)

		cache.PeerIdString(pid1)
		pidStr := cache.PeerIdString(pid2)
		assert.NotEmpty(t, pidStr)
		assert.Equal(t, pid2.String(), pidStr)

		gotPidStr, ok := cache.ByPeerId(pid2)
		assert.True(t, ok, "expected pid to be in the cache")
		assert.Equal(t, pid2.String(), gotPidStr)

		gotPidStr, ok = cache.ByPeerId(pid1)
		assert.True(t, ok, "expected pid to be in the cache")
		assert.Equal(t, pid1.String(), gotPidStr)
	})
}

// TestPeerIdCache_EjectionScenarios tests the eviction logic of the peer ID cache. It ensures that the cache
// evicts the least recently added peer ID when the cache is full.
func TestPeerIdCache_EjectionScenarios(t *testing.T) {
	cacheSize := 3
	cache, err := internal.NewPeerIdCache(cacheSize)
	assert.NoError(t, err)
	assert.Equal(t, 0, cache.Size())

	// add peer IDs to fill the cache
	pid1 := unittest.PeerIdFixture(t)
	pid2 := unittest.PeerIdFixture(t)
	pid3 := unittest.PeerIdFixture(t)

	cache.PeerIdString(pid1)
	assert.Equal(t, 1, cache.Size())
	cache.PeerIdString(pid2)
	assert.Equal(t, 2, cache.Size())
	cache.PeerIdString(pid3)
	assert.Equal(t, 3, cache.Size())

	// check that all peer IDs are in the cache
	assert.Equal(t, pid1.String(), cache.PeerIdString(pid1))
	assert.Equal(t, pid2.String(), cache.PeerIdString(pid2))
	assert.Equal(t, pid3.String(), cache.PeerIdString(pid3))
	assert.Equal(t, 3, cache.Size())

	// add a new peer ID
	pid4 := unittest.PeerIdFixture(t)
	cache.PeerIdString(pid4)
	assert.Equal(t, 3, cache.Size())

	// check that pid1 is now the one that has been evicted
	gotId1Str, ok := cache.ByPeerId(pid1)
	assert.False(t, ok, "expected pid1 to be evicted")
	assert.Equal(t, "", gotId1Str)

	// confirm other peer IDs are still in the cache
	gotId2Str, ok := cache.ByPeerId(pid2)
	assert.True(t, ok, "expected pid2 to be in the cache")
	assert.Equal(t, pid2.String(), gotId2Str)

	gotId3Str, ok := cache.ByPeerId(pid3)
	assert.True(t, ok, "expected pid3 to be in the cache")
	assert.Equal(t, pid3.String(), gotId3Str)

	gotId4Str, ok := cache.ByPeerId(pid4)
	assert.True(t, ok, "expected pid4 to be in the cache")
	assert.Equal(t, pid4.String(), gotId4Str)
}
