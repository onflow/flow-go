package connection

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	discoveryBackoff "github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	ConnectionPruningEnabled  = true
	ConnectionPruningDisabled = false
)

// Libp2pConnector is a libp2p based Connector implementation to connect and disconnect from peers
type Libp2pConnector struct {
	backoffConnector *discoveryBackoff.BackoffConnector
	host             host.Host
	log              zerolog.Logger
	pruneConnections bool
}

var _ p2p.Connector = &Libp2pConnector{}

// UnconvertibleIdentitiesError is an error which reports all the flow.Identifiers that could not be converted to
// peer.AddrInfo
type UnconvertibleIdentitiesError struct {
	errs map[flow.Identifier]error
}

func NewUnconvertableIdentitiesError(errs map[flow.Identifier]error) error {
	return UnconvertibleIdentitiesError{
		errs: errs,
	}
}

func (e UnconvertibleIdentitiesError) Error() string {
	multierr := new(multierror.Error)
	for id, err := range e.errs {
		multierr = multierror.Append(multierr, fmt.Errorf("failed to connect to %s: %w", id.String(), err))
	}
	return multierr.Error()
}

// IsUnconvertibleIdentitiesError returns whether the given error is an UnconvertibleIdentitiesError error
func IsUnconvertibleIdentitiesError(err error) bool {
	var errUnconvertableIdentitiesError UnconvertibleIdentitiesError
	return errors.As(err, &errUnconvertableIdentitiesError)
}

func NewLibp2pConnector(log zerolog.Logger, host host.Host, pruning bool) (*Libp2pConnector, error) {
	connector, err := defaultLibp2pBackoffConnector(host)
	if err != nil {
		return nil, fmt.Errorf("failed to create libP2P connector: %w", err)
	}
	libP2PConnector := &Libp2pConnector{
		log:              log,
		backoffConnector: connector,
		host:             host,
		pruneConnections: pruning,
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

	// stuff all the peer.AddrInfo it into the channel
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

	// get all current node connections
	allCurrentConns := l.host.Network().Conns()

	// for each connection, check if that connection should be trimmed
	for _, conn := range allCurrentConns {

		// get the remote peer ID for this connection
		peerID := conn.RemotePeer()

		// check if the peer ID is included in the current fanout
		if peersToKeep[peerID] {
			continue // skip pruning
		}

		peerInfo := l.host.Network().Peerstore().PeerInfo(peerID)
		lg := l.log.With().Str("remote_peer", peerInfo.String()).Logger()

		// log the protected status of the connection
		protected := l.host.ConnManager().IsProtected(peerID, "")
		lg = lg.With().Bool("protected", protected).Logger()

		// log if any stream is open on this connection.
		flowStream := p2putils.FlowStream(conn)
		if flowStream != nil {
			lg = lg.With().Str("flow_stream", string(flowStream.Protocol())).Logger()
		}

		// close the connection with the peer if it is not part of the current fanout
		err := l.host.Network().ClosePeer(peerID)
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
			Str("connectedness", l.host.Network().Connectedness(peerID).String()).
			Msg("disconnected from peer")
	}
}

// defaultLibp2pBackoffConnector creates a default libp2p backoff connector similar to the one created by libp2p.pubsub
// (https://github.com/libp2p/go-libp2p-pubsub/blob/master/discovery.go#L34)
func defaultLibp2pBackoffConnector(host host.Host) (*discoveryBackoff.BackoffConnector, error) {
	rngSrc := rand.NewSource(rand.Int63())
	minBackoff, maxBackoff := time.Second*10, time.Hour
	cacheSize := 100
	dialTimeout := time.Minute * 2
	backoff := discoveryBackoff.NewExponentialBackoff(minBackoff, maxBackoff, discoveryBackoff.FullJitter, time.Second, 5.0, 0, rand.New(rngSrc))
	backoffConnector, err := discoveryBackoff.NewBackoffConnector(host, cacheSize, dialTimeout, backoff)
	if err != nil {
		return nil, fmt.Errorf("failed to create backoff connector: %w", err)
	}
	return backoffConnector, nil
}
