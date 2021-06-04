package p2p

import (
	"context"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	swarmt "github.com/libp2p/go-libp2p-swarm/testing"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
)

// TestPing tests PingService by creating two libp2p hosts and ping each one from the other
func TestPing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h1 := bhost.New(swarmt.GenSwarm(t, ctx))
	h2 := bhost.New(swarmt.GenSwarm(t, ctx))

	err := h1.Connect(ctx, peer.AddrInfo{
		ID:    h2.ID(),
		Addrs: h2.Addrs(),
	})
	require.NoError(t, err)

	logger := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	protocolID := generatePingProtcolID("1234")
	pingInfoProvider, version, height := MockPingInfoProvider()

	ps1 := NewPingService(h1, protocolID, pingInfoProvider, logger)
	ps2 := NewPingService(h2, protocolID, pingInfoProvider, logger)

	testPing(t, ps1, h2.ID(), version, height)
	testPing(t, ps2, h1.ID(), version, height)
}

func testPing(t *testing.T, ps *PingService, p peer.ID, expectedVersion string, expectedHeight uint64) {
	pctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, rtt, err := ps.Ping(pctx, p)
	assert.NoError(t, err)
	assert.NotZero(t, rtt)
	assert.Equal(t, expectedVersion, resp.Version)
	assert.Equal(t, expectedHeight, resp.BlockHeight)
}
