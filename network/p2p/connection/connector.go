package connection

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	discoveryBackoff "github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/rand"
)

const (
	// PruningEnabled is a boolean flag to enable pruning of connections to peers that are not part of
	// the explicit update list.
	// If set to true, the connector will prune connections to peers that are not part of the explicit update list.
	PruningEnabled = true

	// PruningDisabled is a boolean flag to disable pruning of connections to peers that are not part of
	// the explicit update list.
	// If set to false, the connector will not prune connections to peers that are not part of the explicit update list.
	PruningDisabled = false
)

// Libp2pConnector is a libp2p based Connector implementation to connect and disconnect from peers
type Libp2pConnector struct {
	backoffConnector *discoveryBackoff.BackoffConnector
	host             p2p.ConnectorHost
	log              zerolog.Logger
	pruneConnections bool
}

// ConnectorConfig is the configuration for the libp2p based connector.
type ConnectorConfig struct {
	// PruneConnections is a boolean flag to enable pruning of connections to peers that are not part of the explicit update list.
	PruneConnections bool

	// Logger is the logger to be used by the connector
	Logger zerolog.Logger

	// Host is the libp2p host to be used by the connector.
	Host p2p.ConnectorHost

	// BackoffConnectorFactory is a factory function to create a new BackoffConnector.
	BackoffConnectorFactory func() (*discoveryBackoff.BackoffConnector, error)
}

var _ p2p.Connector = &Libp2pConnector{}

// NewLibp2pConnector creates a new libp2p based connector
// Args:
//   - cfg: configuration for the connector
//
// Returns:
//   - *Libp2pConnector: a new libp2p based connector
//   - error: an error if there is any error while creating the connector. The errors are irrecoverable and unexpected.
func NewLibp2pConnector(cfg *ConnectorConfig) (*Libp2pConnector, error) {
	connector, err := cfg.BackoffConnectorFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create libP2P connector: %w", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create peer ID slice shuffler: %w", err)
	}

	libP2PConnector := &Libp2pConnector{
		log:              cfg.Logger,
		backoffConnector: connector,
		host:             cfg.Host,
		pruneConnections: cfg.PruneConnections,
	}

	return libP2PConnector, nil
}

// UpdatePeers is the implementation of the Connector.UpdatePeers function. It connects to all of the ids and
// disconnects from any other connection that the libp2p node might have.
func (l *Libp2pConnector) UpdatePeers(ctx context.Context, peerIDs peer.IDSlice) {
	// connect to each of the peer.AddrInfo in pInfos
	l.connectToPeers(ctx, peerIDs)

	if l.pruneConnections {
		// disconnect from any other peers not in pInfos
		// Note: by default almost on all roles, we run on a full topology,
		// this trimming only affects evicted peers from protocol state.
		l.pruneAllConnectionsExcept(peerIDs)
	}
}

// connectToPeers connects each of the peer in pInfos
func (l *Libp2pConnector) connectToPeers(ctx context.Context, peerIDs peer.IDSlice) {

	// create a channel of peer.AddrInfo as expected by the connector
	peerCh := make(chan peer.AddrInfo, len(peerIDs))

	// first shuffle, and then stuff all the peer.AddrInfo it into the channel.
	// shuffling is not in place.
	err := rand.Shuffle(uint(len(peerIDs)), func(i, j uint) {
		peerIDs[i], peerIDs[j] = peerIDs[j], peerIDs[i]
	})
	if err != nil {
		// this should never happen, but if it does, we should crash.
		l.log.Fatal().Err(err).Msg("failed to shuffle peer IDs")
	}

	for _, peerID := range peerIDs {
		peerCh <- peer.AddrInfo{ID: peerID}
	}

	// close the channel to ensure Connect does not block
	close(peerCh)

	// ask the connector to connect to all the peers
	l.backoffConnector.Connect(ctx, peerCh)
}

// pruneAllConnectionsExcept trims all connections of the node from peers not part of peerIDs.
// A node would have created such extra connections earlier when the identity list may have been different, or
// it may have been target of such connections from node which have now been excluded.
func (l *Libp2pConnector) pruneAllConnectionsExcept(peerIDs peer.IDSlice) {
	// convert the peerInfos to a peer.ID -> bool map
	peersToKeep := make(map[peer.ID]bool, len(peerIDs))
	for _, pid := range peerIDs {
		peersToKeep[pid] = true
	}

	// for each connection, check if that connection should be trimmed
	for _, conn := range l.host.Connections() {

		// get the remote peer ID for this connection
		peerID := conn.RemotePeer()

		// check if the peer ID is included in the current fanout
		if peersToKeep[peerID] {
			continue // skip pruning
		}

		peerInfo := l.host.PeerInfo(peerID)
		lg := l.log.With().Str("remote_peer", peerInfo.String()).Logger()

		// log the protected status of the connection
		protected := l.host.IsProtected(peerID)
		lg = lg.With().Bool("protected", protected).Logger()

		// log if any stream is open on this connection.
		flowStream := p2putils.FlowStream(conn)
		if flowStream != nil {
			lg = lg.With().Str("flow_stream", string(flowStream.Protocol())).Logger()
		}

		// close the connection with the peer if it is not part of the current fanout
		err := l.host.ClosePeer(peerID)
		if err != nil {
			// logging with suspicious level as failure to disconnect from a peer can be a security issue.
			// e.g., failure to disconnect from a malicious peer can lead to a DoS attack.
			lg.Error().
				Bool(logging.KeySuspicious, true).
				Err(err).Msg("failed to disconnect from peer")
			continue
		}
		// logging with suspicious level as we only expect to disconnect from a peer if it is not part of the
		// protocol state.
		lg.Warn().
			Bool(logging.KeySuspicious, true).
			Msg("disconnected from peer")
	}
}
