package unicast

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
)

// MaxConnectAttemptSleepDuration is the maximum number of milliseconds to wait between attempts for a 1-1 direct connection
const (
	MaxConnectAttemptSleepDuration = 5

	// DefaultRetryDelay is the default initial delay used in the exponential backoff create stream retries while
	// waiting for dialing to peer to be complete
	DefaultRetryDelay = 1 * time.Second
)

var (
	_ p2p.UnicastManager = (*Manager)(nil)
)

// Manager manages libp2p stream negotiation and creation, which is utilized for unicast dispatches.
type Manager struct {
	lock                   sync.RWMutex
	logger                 zerolog.Logger
	streamFactory          StreamFactory
	unicasts               []protocols.Protocol
	defaultHandler         libp2pnet.StreamHandler
	sporkId                flow.Identifier
	connStatus             p2p.PeerConnections
	peerDialing            map[peer.ID]struct{}
	createStreamRetryDelay time.Duration
}

func NewUnicastManager(logger zerolog.Logger, streamFactory StreamFactory, sporkId flow.Identifier, createStreamRetryDelay time.Duration, connStatus p2p.PeerConnections) *Manager {
	return &Manager{
		lock:                   sync.RWMutex{},
		logger:                 logger.With().Str("module", "unicast-manager").Logger(),
		streamFactory:          streamFactory,
		sporkId:                sporkId,
		connStatus:             connStatus,
		peerDialing:            make(map[peer.ID]struct{}),
		createStreamRetryDelay: createStreamRetryDelay,
	}
}

// WithDefaultHandler sets the default stream handler for this unicast manager. The default handler is utilized
// as the core handler for other unicast protocols, e.g., compressions.
func (m *Manager) WithDefaultHandler(defaultHandler libp2pnet.StreamHandler) {
	defaultProtocolID := protocols.FlowProtocolID(m.sporkId)
	m.defaultHandler = defaultHandler

	if len(m.unicasts) > 0 {
		panic("default handler must be set only once before any unicast registration")
	}

	m.unicasts = []protocols.Protocol{
		&PlainStream{
			protocolId: defaultProtocolID,
			handler:    defaultHandler,
		},
	}

	m.streamFactory.SetStreamHandler(defaultProtocolID, defaultHandler)
	m.logger.Info().Str("protocol_id", string(defaultProtocolID)).Msg("default unicast handler registered")
}

// Register registers given protocol name as preferred unicast. Each invocation of register prioritizes the current protocol
// over previously registered ones.ddda
func (m *Manager) Register(unicast protocols.ProtocolName) error {
	factory, err := protocols.ToProtocolFactory(unicast)
	if err != nil {
		return fmt.Errorf("could not translate protocol name into factory: %w", err)
	}

	u := factory(m.logger, m.sporkId, m.defaultHandler)

	m.unicasts = append(m.unicasts, u)
	m.streamFactory.SetStreamHandler(u.ProtocolId(), u.Handler)
	m.logger.Info().Str("protocol_id", string(u.ProtocolId())).Msg("unicast handler registered")

	return nil
}

// CreateStream tries establishing a libp2p stream to the remote peer id. It tries creating streams in the descending order of preference until
// it either creates a successful stream or runs out of options. Creating stream on each protocol is tried at most `maxAttempt` one, and then falls
// back to the less preferred one.
func (m *Manager) CreateStream(ctx context.Context, peerID peer.ID, maxAttempts int) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {
	var errs error
	for i := len(m.unicasts) - 1; i >= 0; i-- {
		// handle the dial in progress error and add retry with backoff collect back off / retry metrics
		s, addrs, err := m.tryCreateStream(ctx, peerID, maxAttempts, m.unicasts[i])
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		// return first successful stream
		return s, addrs, nil
	}

	return nil, nil, fmt.Errorf("could not create stream on any available unicast protocol: %w", errs)
}

// createStream creates a stream to the peerID with the provided unicastProtocol.
func (m *Manager) createStream(ctx context.Context, peerID peer.ID, maxAttempts int, unicastProtocol protocols.Protocol) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {
	s, addrs, err := m.rawStreamWithProtocol(ctx, unicastProtocol.ProtocolId(), peerID, maxAttempts)
	if err != nil {
		return nil, nil, err
	}

	s, err = unicastProtocol.UpgradeRawStream(s)
	if err != nil {
		return nil, nil, err
	}

	return s, addrs, nil

}

// tryCreateStream will retry createStream with the configured exponential backoff delay and maxAttempts. If no stream can be created after max attempts the error is returned.
func (m *Manager) tryCreateStream(ctx context.Context, peerID peer.ID, maxAttempts int, unicastProtocol protocols.Protocol) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {
	var s libp2pnet.Stream
	var addrs []multiaddr.Multiaddr // address on which we dial peerID

	// configure back off retry delay values
	backoff := retry.NewExponential(m.createStreamRetryDelay)
	backoff = retry.WithMaxRetries(uint64(maxAttempts), backoff)

	f := func(context.Context) error {
		var err error
		s, addrs, err = m.createStream(ctx, peerID, maxAttempts, unicastProtocol)
		if err != nil {
			if IsErrDialInProgress(err) {
				// capture dial in progress metric and log
				return retry.RetryableError(err)
			}
			return err
		}

		return nil
	}

	err := retry.Do(ctx, backoff, f)
	if err != nil {
		return nil, nil, err
	}

	return s, addrs, nil
}

// rawStreamWithProtocol creates a stream raw libp2p stream on specified protocol.
//
// Note: a raw stream must be upgraded by the given unicast protocol id.
//
// It makes at most `maxAttempts` to create a stream with the peer.
// This was put in as a fix for #2416. PubSub and 1-1 communication compete with each other when trying to connect to
// remote nodes and once in a while NewStream returns an error 'both yamux endpoints are clients'.
//
// Note that in case an existing TCP connection underneath to `peerID` exists, that connection is utilized for creating a new stream.
// The multiaddr.Multiaddr return value represents the addresses of `peerID` we dial while trying to create a stream to it.
// Expected errors during normal operations:
//   - ErrDialInProgress if no connection to the peer exists and there is already a dial in progress to the peer. If a dial to
//     the peer is already in progress the caller needs to wait until it is completed, a peer should be dialed only once.
//
// Unexpected errors during normal operations:
//   - network.ErrUnexpectedConnectionStatus indicates bug in libpp2p when checking IsConnected status of peer.
func (m *Manager) rawStreamWithProtocol(ctx context.Context,
	protocolID protocol.ID,
	peerID peer.ID,
	maxAttempts int) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {

	var errs error
	var s libp2pnet.Stream
	var retries = 0
	var dialAddr []multiaddr.Multiaddr // address on which we dial peerID
	for ; retries < maxAttempts; retries++ {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("context done before stream could be created (retry attempt: %d, errors: %w)", retries, errs)
		default:
		}

		// libp2p internally uses swarm dial - https://github.com/libp2p/go-libp2p-swarm/blob/master/swarm_dial.go
		// to connect to a peer. Swarm dial adds a back off each time it fails connecting to a peer. While this is
		// the desired behaviour for pub-sub (1-k style of communication) for 1-1 style we want to retry the connection
		// immediately without backing off and fail-fast.
		// Hence, explicitly cancel the dial back off (if any) and try connecting again

		isConnected, err := m.connStatus.IsConnected(peerID)
		if err != nil {
			return nil, nil, err
		}

		// dial peer and establish connection if one does not exist
		if !isConnected {
			// we prevent nodes from dialingComplete peers multiple times which leads to multiple connections being
			// created under the hood which can lead to resource exhaustion
			if m.isDialing(peerID) {
				return nil, nil, NewDialInProgressErr(peerID)
			}

			m.dialingInProgress(peerID)
			// cancel the dial back off (if any), since we want to connect immediately
			dialAddr = m.streamFactory.DialAddress(peerID)
			m.streamFactory.ClearBackoff(peerID)

			// if this is a retry attempt, wait for some time before retrying
			if retries > 0 {
				// choose a random interval between 0 to 5
				// (to ensure that this node and the target node don't attempt to reconnect at the same time)
				r := rand.Intn(MaxConnectAttemptSleepDuration)
				time.Sleep(time.Duration(r) * time.Millisecond)
			}

			err := m.streamFactory.Connect(ctx, peer.AddrInfo{ID: peerID})
			if err != nil {
				// if the connection was rejected due to invalid node id, skip the re-attempt
				if strings.Contains(err.Error(), "failed to negotiate security protocol") {
					m.dialingComplete(peerID)
					return s, dialAddr, fmt.Errorf("invalid node id: %w", err)
				}

				// if the connection was rejected due to allowlisting, skip the re-attempt
				if errors.Is(err, swarm.ErrGaterDisallowedConnection) {
					m.dialingComplete(peerID)
					return s, dialAddr, fmt.Errorf("target node is not on the approved list of nodes: %w", err)
				}

				errs = multierror.Append(errs, err)
				continue
			}
			m.dialingComplete(peerID)
		}

		// add libp2p context value NoDial to prevent the underlying host from dialingComplete the peer while creating the stream
		// we've already ensured that a connection already exists.
		ctx = libp2pnet.WithNoDial(ctx, "application ensured connection to peer exists")
		// creates stream using stream factory
		s, err = m.streamFactory.NewStream(ctx, peerID, protocolID)
		if err != nil {
			// if the stream creation failed due to invalid protocol id, skip the re-attempt
			if strings.Contains(err.Error(), "protocol not supported") {
				return nil, dialAddr, fmt.Errorf("remote node is running on a different spork: %w, protocol attempted: %s", err, protocolID)
			}
			errs = multierror.Append(errs, err)
			continue
		}

		break
	}

	if retries == maxAttempts {
		return s, dialAddr, errs
	}

	return s, dialAddr, nil
}

// isDialing returns true if dialing to peer in progress.
func (m *Manager) isDialing(peerID peer.ID) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	_, ok := m.peerDialing[peerID]
	return ok
}

// dialingInProgress sets peerDialing value for peerID indicating dialing in progress.
func (m *Manager) dialingInProgress(peerID peer.ID) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.peerDialing[peerID] = struct{}{}
}

// dialingComplete removes peerDialing value for peerID indicating dialing to peerID no longer in progress.
func (m *Manager) dialingComplete(peerID peer.ID) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.peerDialing, peerID)
}
