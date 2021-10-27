package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolPeerCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create three hosts, and a pcache for the first
	h1, err := DefaultLibP2PHost(ctx, "0.0.0.0:0", unittest.KeyFixture(fcrypto.ECDSASecp256k1))
	require.NoError(t, err)
	pcache, err := newProtocolPeerCache(zerolog.Nop(), h1)
	require.NoError(t, err)
	h2, err := DefaultLibP2PHost(ctx, "0.0.0.0:0", unittest.KeyFixture(fcrypto.ECDSASecp256k1))
	require.NoError(t, err)
	h3, err := DefaultLibP2PHost(ctx, "0.0.0.0:0", unittest.KeyFixture(fcrypto.ECDSASecp256k1))
	require.NoError(t, err)

	// register each host on a separate protocol
	p1 := protocol.ID("p1")
	p2 := protocol.ID("p2")
	p3 := protocol.ID("p3")
	noopHandler := func(s network.Stream) {}
	h1.SetStreamHandler(p1, noopHandler)
	h2.SetStreamHandler(p2, noopHandler)
	h3.SetStreamHandler(p3, noopHandler)

	// connect the hosts to each other
	require.NoError(t, h1.Connect(ctx, *host.InfoFromHost(h2)))
	require.NoError(t, h1.Connect(ctx, *host.InfoFromHost(h3)))
	require.NoError(t, h2.Connect(ctx, *host.InfoFromHost(h3)))

	// check that h1's pcache reflects the protocols supported by h2 and h3
	assert.Eventually(t, func() bool {
		peers2 := pcache.getPeers(p2)
		peers3 := pcache.getPeers(p3)
		_, ok2 := peers2[h2.ID()]
		_, ok3 := peers3[h3.ID()]
		return len(peers2) == 1 && len(peers3) == 1 && ok2 && ok3
	}, 3*time.Second, 50*time.Millisecond)

	// remove h2's support for p2
	h2.RemoveStreamHandler(p2)

	// check that h1's pcache reflects the change
	assert.Eventually(t, func() bool {
		return len(pcache.getPeers(p2)) == 0
	}, 3*time.Second, 50*time.Millisecond)

	// add support for p4 on h2 and h3
	p4 := protocol.ID("p4")
	h2.SetStreamHandler(p4, noopHandler)
	h3.SetStreamHandler(p4, noopHandler)

	// check that h1's pcache reflects the change
	assert.Eventually(t, func() bool {
		peers4 := pcache.getPeers(p4)
		_, ok2 := peers4[h2.ID()]
		_, ok3 := peers4[h3.ID()]
		return len(peers4) == 2 && ok2 && ok3
	}, 3*time.Second, 50*time.Millisecond)
}
