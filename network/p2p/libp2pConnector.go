package p2p

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

// libp2pConnector is a libp2p based Connector implementation to connect and disconnect from peers
type Libp2pConnector struct {
	backoffConnector *discovery.BackoffConnector
	host             host.Host
	log              zerolog.Logger
	pruneConnections bool
}

var _ Connector = &Libp2pConnector{}

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

type ConnectorOption func(connector *Libp2pConnector)

func WithConnectionPruning(enable bool) ConnectorOption {
	return func(connector *Libp2pConnector) {
		connector.pruneConnections = false
	}
}

func NewLibp2pConnector(host host.Host, log zerolog.Logger, options ...ConnectorOption) (*Libp2pConnector, error) {
	connector, err := defaultLibp2pBackoffConnector(host)
	if err != nil {
		return nil, fmt.Errorf("failed to create libP2P connector: %w", err)
	}
	libP2PConnector := &Libp2pConnector{
		backoffConnector: connector,
		host:             host,
		log:              log,
		pruneConnections: true,
	}

	for _, o := range options {
		o(libP2PConnector)
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
		l.trimAllConnectionsExcept(peerIDs)
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

// trimAllConnectionsExcept trims all connections of the node from peers not part of peerIDs.
// A node would have created such extra connections earlier when the identity list may have been different, or
// it may have been target of such connections from node which have now been excluded.
func (l *Libp2pConnector) trimAllConnectionsExcept(peerIDs peer.IDSlice) {

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
		log := l.log.With().Str("remote_peer", peerInfo.String()).Logger()
		if l.host.ConnManager().IsProtected(peerID, "") {
			log.Trace().Msg("skipping pruning since connection is protected")
			continue // connection is protected (stream or connection in progress), skip pruning
		}

		// retain the connection if there is a Flow One-to-One stream on that connection
		// (we do not want to sever a connection with on going direct one-to-one traffic)
		flowStream := flowStream(conn)
		if flowStream != nil {
			log.Info().
				Str("stream_protocol", string(flowStream.Protocol())).
				Msg("skipping connection pruning with peer due to one-to-one stream")
			continue // flow stream found, skip pruning
		}

		// close the connection with the peer if it is not part of the current fanout
		err := l.host.Network().ClosePeer(peerID)
		if err != nil {
			log.Error().Err(err).Msg("failed to disconnect from peer")
		} else {
			log.Debug().Msg("disconnected from peer not included in the fanout")
		}
	}
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
