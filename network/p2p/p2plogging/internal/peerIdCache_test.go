package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/network/p2p/p2plogging/internal"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
)

func TestNewPeerIdCache(t *testing.T) {
	cacheSize := uint32(100)
	cache := internal.NewPeerIdCache(cacheSize)
	assert.NotNil(t, cache)
}

func TestPeerIdCache_PeerIdString(t *testing.T) {
	cacheSize := uint32(100)
	cache := internal.NewPeerIdCache(cacheSize)

	t.Run("existing peer ID", func(t *testing.T) {
		pid := p2ptest.PeerIdFixture(t)
		pidStr := cache.PeerIdString(pid)

		assert.NotEmpty(t, pidStr)
		assert.Equal(t, pid.String(), pidStr)
	})

	t.Run("non-existing peer ID", func(t *testing.T) {
		pid1 := p2ptest.PeerIdFixture(t)
		pid2 := p2ptest.PeerIdFixture(t)

		cache.PeerIdString(pid1)
		pidStr := cache.PeerIdString(pid2)

		assert.NotEmpty(t, pidStr)
		assert.Equal(t, pid2.String(), pidStr)
	})
}
