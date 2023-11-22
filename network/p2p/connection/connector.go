package connection

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
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

// PeerUpdater is a connector that connects to a list of peers and disconnects from any other connection that the libp2p node might have.
type PeerUpdater struct {
	connector        p2p.Connector
	host             p2p.ConnectorHost
	log              zerolog.Logger
	pruneConnections bool
}

// PeerUpdaterConfig is the configuration for the libp2p based connector.
type PeerUpdaterConfig struct {
	// PruneConnections is a boolean flag to enable pruning of connections to peers that are not part of the explicit update list.
	PruneConnections bool

	// Logger is the logger to be used by the connector
	Logger zerolog.Logger

	// Host is the libp2p host to be used by the connector.
	Host p2p.ConnectorHost

	// ConnectorFactory is a factory function to create a new connector.
	Connector p2p.Connector
}

var _ p2p.PeerUpdater = (*PeerUpdater)(nil)

// NewPeerUpdater creates a new libp2p based connector
// Args:
//   - cfg: configuration for the connector
//
// Returns:
//   - *PeerUpdater: a new libp2p based connector
//   - error: an error if there is any error while creating the connector. The errors are irrecoverable and unexpected.
func NewPeerUpdater(cfg *PeerUpdaterConfig) (*PeerUpdater, error) {
	libP2PConnector := &PeerUpdater{
		log:              cfg.Logger.With().Str("component", "peer-updater").Logger(),
		connector:        cfg.Connector,
		host:             cfg.Host,
		pruneConnections: cfg.PruneConnections,
	}

	return libP2PConnector, nil
}

// UpdatePeers is the implementation of the Connector.UpdatePeers function. It connects to all of the ids and
// disconnects from any other connection that the libp2p node might have.
func (l *PeerUpdater) UpdatePeers(ctx context.Context, peerIDs peer.IDSlice) {
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
func (l *PeerUpdater) connectToPeers(ctx context.Context, peerIDs peer.IDSlice) {

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
		if l.host.IsConnectedTo(peerID) {
			l.log.Trace().Str("peer_id", p2plogging.PeerId(peerID)).Msg("already connected to peer, skipping connection")
			continue
		}
		peerCh <- peer.AddrInfo{ID: peerID}
	}

	// close the channel to ensure Connect does not block
	close(peerCh)

	// ask the connector to connect to all the peers
	l.connector.Connect(ctx, peerCh)
}

// pruneAllConnectionsExcept trims all connections of the node from peers not part of peerIDs.
// A node would have created such extra connections earlier when the identity list may have been different, or
// it may have been target of such connections from node which have now been excluded.
func (l *PeerUpdater) pruneAllConnectionsExcept(peerIDs peer.IDSlice) {
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
		for _, stream := range conn.GetStreams() {
			if err := stream.Close(); err != nil {
				lg.Warn().Err(err).Msg("failed to close stream, when pruning connections")
			}
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
