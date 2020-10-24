package libp2p

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"

	"github.com/onflow/flow-go/model/flow"
)

// libp2pConnector is a libp2p based Connector implementation to connect and disconnect from peers
type libp2pConnector struct {
	backoffConnector *discovery.BackoffConnector
	host             host.Host
}

var _ Connector = &libp2pConnector{}

func NewLibp2pConnector(host host.Host) (*libp2pConnector, error) {
	connector, err := defaultLibp2pBackoffConnector(host)
	if err != nil {
		return nil, fmt.Errorf("failed to create libP2P connector: %w", err)
	}
	return &libp2pConnector{
		backoffConnector: connector,
		host:             host,
	}, nil
}

func (l *libp2pConnector) ConnectPeers(ctx context.Context, ids flow.IdentityList) map[flow.Identifier]error {

	validIDs, invalidIDs := peerInfosFromIDs(ids)

	// create a channel of peer.AddrInfo as expected by the connector
	peerCh := make(chan peer.AddrInfo, len(validIDs))

	// stuff all the peer.AddrInfo it into the channel
	for _, peerInfo := range validIDs {
		peerCh <- peerInfo
	}

	// close the channel to ensure Connect does not block
	close(peerCh)

	// ask the connector to connect to all the peers
	l.backoffConnector.Connect(ctx, peerCh)

	return invalidIDs
}

func (l *libp2pConnector) DisconnectPeers(ctx context.Context, ids flow.IdentityList) map[flow.Identifier]error {

	validIDs, invalidIDs := peerInfosFromIDs(ids)

	// disconnect from each of the peer.AddrInfo
	for id, peerInfo := range validIDs {
		if l.isConnected(peerInfo) {
			err := l.host.Network().ClosePeer(peerInfo.ID)
			if err != nil {
				invalidIDs[id] = err
			}
		}
	}

	return invalidIDs
}

func (l *libp2pConnector) isConnected(peerInfo peer.AddrInfo) bool {
	connectedness := l.host.Network().Connectedness(peerInfo.ID)
	return connectedness == network.Connected
}

// defaultLibp2pBackoffConnector creates a default libp2p backoff connector similar to the one created by libp2p.pubsub
// (https://github.com/libp2p/go-libp2p-pubsub/blob/master/discovery.go#L34)
func defaultLibp2pBackoffConnector(host host.Host) (*discovery.BackoffConnector, error) {
	rngSrc := rand.NewSource(rand.Int63())
	minBackoff, maxBackoff := time.Second*10, time.Hour
	cacheSize := 100
	dialTimeout := time.Minute * 2
	backoff := discovery.NewExponentialBackoff(minBackoff, maxBackoff, discovery.FullJitter, time.Second, 5.0, 0, rand.New(rngSrc))
	backoffConnector, err := discovery.NewBackoffConnector(host, cacheSize, dialTimeout, backoff)
	if err != nil {
		return nil, fmt.Errorf("failed to create backoff connector: %w", err)
	}
	return backoffConnector, nil
}

// peerInfosFromIDs converts the given flow.Identities to peer.AddrInfo.
// If the conversion of flow.Identifier succeeds it is included in map[flow.Identifier]peer.AddrInfo else it included
// in map[flow.Identifier]error.
func peerInfosFromIDs(ids flow.IdentityList) (map[flow.Identifier]peer.AddrInfo, map[flow.Identifier]error) {
	validIDs := make(map[flow.Identifier]peer.AddrInfo)
	invalidIDs := make(map[flow.Identifier]error)
	for _, id := range ids {
		peerInfo, err := PeerInfoFromID(*id)
		if err != nil {
			invalidIDs[id.NodeID] = err
			continue
		}
		validIDs[id.NodeID] = peerInfo
	}
	return validIDs, invalidIDs
}
