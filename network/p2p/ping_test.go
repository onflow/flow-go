package p2p

import (
	"context"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

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

	if err != nil {
		t.Fatal(err)
	}

	logger := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	protocolID := generatePingProtcolID("1234")
	version := "version_1"
	ps1 := NewPingService(h1, protocolID, version, logger)
	ps2 := NewPingService(h2, protocolID, version, logger)

	testPing(t, ps1, h2.ID(), version)
	testPing(t, ps2, h1.ID(), version)
}

func testPing(t *testing.T, ps *PingService, p peer.ID, expectedVersion string) {
	pctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, rtt, err := ps.Ping(pctx, p)
	assert.NoError(t, err)
	assert.NotZero(t, rtt)
	assert.Equal(t, expectedVersion, resp.Version)
}
