package p2ptest

import (
	"context"
	"time"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
)

// UnicastManagerFixture unicast manager fixture that can be used to override the default unicast manager for libp2p nodes.
type UnicastManagerFixture struct {
	*unicast.Manager
}

// UnicastManagerFixtureFactory returns a new UnicastManagerFixture.
func UnicastManagerFixtureFactory() p2p.UnicastManagerFactoryFunc {
	return func(logger zerolog.Logger,
		streamFactory stream.Factory,
		sporkId flow.Identifier,
		createStreamRetryDelay time.Duration,
		connStatus p2p.PeerConnections,
		metrics module.UnicastManagerMetrics) p2p.UnicastManager {
		uniMgr := unicast.NewUnicastManager(logger,
			streamFactory,
			sporkId,
			createStreamRetryDelay,
			connStatus,
			metrics)
		return &UnicastManagerFixture{
			Manager: uniMgr,
		}
	}
}

// CreateStream override the CreateStream func and create streams without retries and without enforcing a single pairwise connection.
func (m *UnicastManagerFixture) CreateStream(ctx context.Context, peerID peer.ID, _ int) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {
	protocol := m.Protocols()[0]
	streamFactory := m.StreamFactory()

	// cancel the dial back off (if any), since we want to connect immediately
	dialAddr := streamFactory.DialAddress(peerID)
	streamFactory.ClearBackoff(peerID)
	ctx = libp2pnet.WithForceDirectDial(ctx, "allow multiple connections")
	err := streamFactory.Connect(ctx, peer.AddrInfo{ID: peerID})
	if err != nil {
		return nil, dialAddr, err
	}

	// creates stream using stream factory
	s, err := streamFactory.NewStream(ctx, peerID, protocol.ProtocolId())
	if err != nil {
		return nil, dialAddr, err
	}

	s, err = protocol.UpgradeRawStream(s)
	if err != nil {
		return nil, dialAddr, err
	}
	return s, dialAddr, nil
}
