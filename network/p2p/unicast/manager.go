package unicast

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
)

const (
	// MaxRetryJitter is the maximum number of milliseconds to wait between attempts for a 1-1 direct connection
	MaxRetryJitter = 5

	// DefaultRetryDelay Initial delay between failing to establish a connection with another node and retrying. This delay
	// increases exponentially (exponential backoff) with the number of subsequent failures to establish a connection.
	DefaultRetryDelay = 1 * time.Second
)

var (
	_ p2p.UnicastManager = (*Manager)(nil)
)

// Manager manages libp2p stream negotiation and creation, which is utilized for unicast dispatches.
type Manager struct {
	logger                 zerolog.Logger
	streamFactory          stream.Factory
	protocols              []protocols.Protocol
	defaultHandler         libp2pnet.StreamHandler
	sporkId                flow.Identifier
	connStatus             p2p.PeerConnections
	peerDialing            sync.Map
	createStreamRetryDelay time.Duration
	metrics                module.UnicastManagerMetrics
}

func NewUnicastManager(logger zerolog.Logger,
	streamFactory stream.Factory,
	sporkId flow.Identifier,
	createStreamRetryDelay time.Duration,
	connStatus p2p.PeerConnections,
	metrics module.UnicastManagerMetrics,
) *Manager {
	return &Manager{
		logger:                 logger.With().Str("module", "unicast-manager").Logger(),
		streamFactory:          streamFactory,
		sporkId:                sporkId,
		connStatus:             connStatus,
		peerDialing:            sync.Map{},
		createStreamRetryDelay: createStreamRetryDelay,
		metrics:                metrics,
	}
}

// WithDefaultHandler sets the default stream handler for this unicast manager. The default handler is utilized
// as the core handler for other unicast protocols, e.g., compressions.
func (m *Manager) WithDefaultHandler(defaultHandler libp2pnet.StreamHandler) {
	defaultProtocolID := protocols.FlowProtocolID(m.sporkId)
	if len(m.protocols) > 0 {
		panic("default handler must be set only once before any unicast registration")
	}

	m.defaultHandler = defaultHandler

	m.protocols = []protocols.Protocol{
		&PlainStream{
			protocolId: defaultProtocolID,
			handler:    defaultHandler,
		},
	}

	m.streamFactory.SetStreamHandler(defaultProtocolID, defaultHandler)
	m.logger.Info().Str("protocol_id", string(defaultProtocolID)).Msg("default unicast handler registered")
}

// Register registers given protocol name as preferred unicast. Each invocation of register prioritizes the current protocol
// over previously registered ones.
func (m *Manager) Register(protocol protocols.ProtocolName) error {
	factory, err := protocols.ToProtocolFactory(protocol)
	if err != nil {
		return fmt.Errorf("could not translate protocol name into factory: %w", err)
	}

	u := factory(m.logger, m.sporkId, m.defaultHandler)

	m.protocols = append(m.protocols, u)
	m.streamFactory.SetStreamHandler(u.ProtocolId(), u.Handler)
	m.logger.Info().Str("protocol_id", string(u.ProtocolId())).Msg("unicast handler registered")

	return nil
}

// CreateStream tries establishing a libp2p stream to the remote peer id. It tries creating streams in the descending order of preference until
// it either creates a successful stream or runs out of options. Creating stream on each protocol is tried at most `maxAttempts`, and then falls
// back to the less preferred one.
func (m *Manager) CreateStream(ctx context.Context, peerID peer.ID, maxAttempts int) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {
	var errs error
	for i := len(m.protocols) - 1; i >= 0; i-- {
		s, addrs, err := m.tryCreateStream(ctx, peerID, uint64(maxAttempts), m.protocols[i])
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		// return first successful stream
		return s, addrs, nil
	}

	return nil, nil, fmt.Errorf("could not create stream on any available unicast protocol: %w", errs)
}

// tryCreateStream will retry createStream with the configured exponential backoff delay and maxAttempts.
// During retries, each error encountered is aggregated in a multierror. If max attempts are made before a
// stream can be successfully the multierror will be returned. During stream creation when IsErrDialInProgress
// is encountered during retries this would indicate that no connection to the peer exists yet.
// In this case we will retry creating the stream with a backoff until a connection is established.
func (m *Manager) tryCreateStream(ctx context.Context, peerID peer.ID, maxAttempts uint64, protocol protocols.Protocol) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {
	var err error
	var s libp2pnet.Stream
	var addrs []multiaddr.Multiaddr // address on which we dial peerID

	// configure back off retry delay values
	backoff := retry.NewExponential(m.createStreamRetryDelay)
	// https://github.com/sethvargo/go-retry#maxretries retries counter starts at zero and library will make last attempt
	// when retries == maxAttempts causing 1 more func invocation than expected.
	maxRetries := maxAttempts - 1
	backoff = retry.WithMaxRetries(maxRetries, backoff)

	attempts := 0
	// retryable func will attempt to create the stream and only retry if dialing the peer is in progress
	f := func(context.Context) error {
		attempts++
		s, addrs, err = m.createStream(ctx, peerID, maxAttempts, protocol)
		if err != nil {
			if IsErrDialInProgress(err) {
				m.logger.Warn().
					Err(err).
					Str("peer_id", peerID.String()).
					Int("attempt", attempts).
					Uint64("max_attempts", maxAttempts).
					Msg("retrying create stream, dial to peer in progress")
				return retry.RetryableError(err)
			}
			return err
		}

		return nil
	}
	start := time.Now()
	err = retry.Do(ctx, backoff, f)
	duration := time.Since(start)
	if err != nil {
		m.metrics.OnStreamCreationFailure(duration, attempts)
		return nil, nil, err
	}

	m.metrics.OnStreamCreated(duration, attempts)
	return s, addrs, nil
}

// createStream creates a stream to the peerID with the provided protocol.
func (m *Manager) createStream(ctx context.Context, peerID peer.ID, maxAttempts uint64, protocol protocols.Protocol) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {
	s, addrs, err := m.rawStreamWithProtocol(ctx, protocol.ProtocolId(), peerID, maxAttempts)
	if err != nil {
		return nil, nil, err
	}

	s, err = protocol.UpgradeRawStream(s)
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
// The multiaddr.Multiaddr return value represents the addresses of `peerID` we dial while trying to create a stream to it, the
// multiaddr is only returned when a peer is initially dialed.
// Expected errors during normal operations:
//   - ErrDialInProgress if no connection to the peer exists and there is already a dial in progress to the peer. If a dial to
//     the peer is already in progress the caller needs to wait until it is completed, a peer should be dialed only once.
//
// Unexpected errors during normal operations:
//   - network.ErrIllegalConnectionState indicates bug in libpp2p when checking IsConnected status of peer.
func (m *Manager) rawStreamWithProtocol(ctx context.Context,
	protocolID protocol.ID,
	peerID peer.ID,
	maxAttempts uint64,
) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {
	isConnected, err := m.connStatus.IsConnected(peerID)
	if err != nil {
		return nil, nil, err
	}

	// check connection status and attempt to dial the peer if dialing is not in progress
	if !isConnected {
		// return error if we can't start dialing
		if m.dialingInProgress(peerID) {
			return nil, nil, NewDialInProgressErr(peerID)
		}
		defer m.dialingComplete(peerID)
		dialAddr, err := m.dialPeer(ctx, peerID, maxAttempts)
		if err != nil {
			return nil, dialAddr, err
		}
	}

	// at this point dialing should have completed, we are already connected we can attempt to create the stream
	s, err := m.rawStream(ctx, peerID, protocolID, maxAttempts)
	if err != nil {
		return nil, nil, err
	}

	return s, nil, nil
}

// dialPeer dial peer with retries.
// Expected errors during normal operations:
//   - ErrMaxRetries if retry attempts are exhausted
func (m *Manager) dialPeer(ctx context.Context, peerID peer.ID, maxAttempts uint64) ([]multiaddr.Multiaddr, error) {
	// aggregated retryable errors that occur during retries, errs will be returned
	// if retry context times out or maxAttempts have been made before a successful retry occurs
	var errs error
	var dialAddr []multiaddr.Multiaddr
	dialAttempts := 0
	backoff := m.retryBackoff(maxAttempts)
	f := func(context.Context) error {
		dialAttempts++
		select {
		case <-ctx.Done():
			return fmt.Errorf("context done before stream could be created (retry attempt: %d, errors: %w)", dialAttempts, errs)
		default:
		}
		// libp2p internally uses swarm dial - https://github.com/libp2p/go-libp2p-swarm/blob/master/swarm_dial.go
		// to connect to a peer. Swarm dial adds a back off each time it fails connecting to a peer. While this is
		// the desired behaviour for pub-sub (1-k style of communication) for 1-1 style we want to retry the connection
		// immediately without backing off and fail-fast.
		// Hence, explicitly cancel the dial back off (if any) and try connecting again

		// cancel the dial back off (if any), since we want to connect immediately
		dialAddr = m.streamFactory.DialAddress(peerID)
		m.streamFactory.ClearBackoff(peerID)
		err := m.streamFactory.Connect(ctx, peer.AddrInfo{ID: peerID})
		if err != nil {
			// if the connection was rejected due to invalid node id or
			// if the connection was rejected due to connection gating skip the re-attempt
			if stream.IsErrSecurityProtocolNegotiationFailed(err) || stream.IsErrGaterDisallowedConnection(err) {
				return multierror.Append(errs, err)
			}
			m.logger.Warn().
				Err(err).
				Str("peer_id", peerID.String()).
				Int("attempt", dialAttempts).
				Uint64("max_attempts", maxAttempts).
				Msg("retrying peer dialing")
			return retry.RetryableError(multierror.Append(errs, err))
		}
		return nil
	}

	start := time.Now()
	err := retry.Do(ctx, backoff, f)
	duration := time.Since(start)
	if err != nil {
		m.metrics.OnPeerDialFailure(duration, dialAttempts)
		return dialAddr, m.retryFailedError(uint64(dialAttempts), maxAttempts, fmt.Errorf("failed to dial peer: %w", err))
	}
	m.metrics.OnPeerDialed(duration, dialAttempts)
	return dialAddr, nil
}

// rawStream creates a stream to peer with retries.
// Expected errors during normal operations:
//   - ErrMaxRetries if retry attempts are exhausted
func (m *Manager) rawStream(ctx context.Context, peerID peer.ID, protocolID protocol.ID, maxAttempts uint64) (libp2pnet.Stream, error) {
	// aggregated retryable errors that occur during retries, errs will be returned
	// if retry context times out or maxAttempts have been made before a successful retry occurs
	var errs error
	var s libp2pnet.Stream
	attempts := 0
	f := func(context.Context) error {
		attempts++
		select {
		case <-ctx.Done():
			return fmt.Errorf("context done before stream could be created (retry attempt: %d, errors: %w)", attempts, errs)
		default:
		}

		var err error
		// add libp2p context value NoDial to prevent the underlying host from dialingComplete the peer while creating the stream
		// we've already ensured that a connection already exists.
		ctx = libp2pnet.WithNoDial(ctx, "application ensured connection to peer exists")
		// creates stream using stream factory
		s, err = m.streamFactory.NewStream(ctx, peerID, protocolID)
		if err != nil {
			// if the stream creation failed due to invalid protocol id, skip the re-attempt
			if stream.IsErrProtocolNotSupported(err) {
				return err
			}
			return retry.RetryableError(multierror.Append(errs, err))
		}
		return nil
	}

	start := time.Now()
	err := retry.Do(ctx, m.retryBackoff(maxAttempts), f)
	duration := time.Since(start)
	if err != nil {
		m.metrics.OnEstablishStreamFailure(duration, attempts)
		return nil, m.retryFailedError(uint64(attempts), maxAttempts, fmt.Errorf("failed to create a stream to peer: %w", err))
	}
	m.metrics.OnStreamEstablished(duration, attempts)
	return s, nil
}

// retryBackoff returns an exponential retry with jitter and max attempts.
func (m *Manager) retryBackoff(maxAttempts uint64) retry.Backoff {
	// create backoff
	backoff := retry.NewConstant(time.Second)
	// add a MaxRetryJitter*time.Millisecond jitter to our backoff to ensure that this node and the target node don't attempt to reconnect at the same time
	backoff = retry.WithJitter(MaxRetryJitter*time.Millisecond, backoff)
	// https://github.com/sethvargo/go-retry#maxretries retries counter starts at zero and library will make last attempt
	// when retries == maxAttempts causing 1 more func invocation than expected.
	maxRetries := maxAttempts - 1
	backoff = retry.WithMaxRetries(maxRetries, backoff)
	return backoff
}

// retryFailedError wraps the given error in a ErrMaxRetries if maxAttempts were made.
func (m *Manager) retryFailedError(dialAttempts, maxAttempts uint64, err error) error {
	if dialAttempts == maxAttempts {
		return NewMaxRetriesErr(dialAttempts, err)
	}
	return err
}

// dialingInProgress sets the value for peerID key in our map if it does not already exist.
func (m *Manager) dialingInProgress(peerID peer.ID) bool {
	_, loaded := m.peerDialing.LoadOrStore(peerID, struct{}{})
	return loaded
}

// dialingComplete removes peerDialing value for peerID indicating dialing to peerID no longer in progress.
func (m *Manager) dialingComplete(peerID peer.ID) {
	m.peerDialing.Delete(peerID)
}
