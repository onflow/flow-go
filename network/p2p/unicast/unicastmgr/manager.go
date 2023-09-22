package unicastmgr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
	"github.com/onflow/flow-go/network/p2p/unicast/unicastmodel"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// MaxRetryJitter is the maximum number of milliseconds to wait between attempts for a 1-1 direct connection
	MaxRetryJitter = 5
)

var (
	_ p2p.UnicastManager = (*Manager)(nil)
)

type DialConfigCacheFactory func(configFactory func() unicastmodel.DialConfig) unicast.DialConfigCache

// Manager manages libp2p stream negotiation and creation, which is utilized for unicast dispatches.
type Manager struct {
	logger                 zerolog.Logger
	streamFactory          p2p.StreamFactory
	protocols              []protocols.Protocol
	defaultHandler         libp2pnet.StreamHandler
	sporkId                flow.Identifier
	connStatus             p2p.PeerConnections
	peerDialing            sync.Map
	createStreamRetryDelay time.Duration
	metrics                module.UnicastManagerMetrics

	// dialConfigCache is a cache to store the dial config for each peer.
	// TODO: encapsulation can be further improved by wrapping the dialConfigCache together with the dial config adjustment logic into a single struct.
	dialConfigCache unicast.DialConfigCache

	// streamZeroBackoffResetThreshold is the threshold that determines when to reset the stream creation backoff budget to the default value.
	//
	// For example the default value of 100 means that if the stream creation backoff budget is decreased to 0, then it will be reset to default value
	// when the number of consecutive successful streams reaches 100.
	//
	// This is to prevent the backoff budget from being reset too frequently, as the backoff budget is used to gauge the reliability of the stream creation.
	// When the stream creation backoff budget is reset to the default value, it means that the stream creation is reliable enough to be trusted again.
	// This parameter mandates when the stream creation is reliable enough to be trusted again; i.e., when the number of consecutive successful streams reaches this threshold.
	// Note that the counter is reset to 0 when the stream creation fails, so the value of for example 100 means that the stream creation is reliable enough that the recent
	// 100 stream creations are all successful.
	streamZeroBackoffResetThreshold uint64

	// dialZeroBackoffResetThreshold is the threshold that determines when to reset the dial backoff budget to the default value.
	//
	// For example the threshold of 1 hour means that if the dial backoff budget is decreased to 0, then it will be reset to default value
	// when it has been 1 hour since the last successful dial.
	//
	// This is to prevent the backoff budget from being reset too frequently, as the backoff budget is used to gauge the reliability of the dialing a remote peer.
	// When the dial backoff budget is reset to the default value, it means that the dialing is reliable enough to be trusted again.
	// This parameter mandates when the dialing is reliable enough to be trusted again; i.e., when it has been 1 hour since the last successful dial.
	// Note that the last dial attempt timestamp is reset to zero when the dial fails, so the value of for example 1 hour means that the dialing to the remote peer is reliable enough that the last
	// successful dial attempt was 1 hour ago.
	dialZeroBackoffResetThreshold time.Duration

	// maxDialAttemptTimes is the maximum number of attempts to be made to connect to a remote node to establish a unicast (1:1) connection before we give up.
	maxDialAttemptTimes uint64

	// maxStreamCreationAttemptTimes is the maximum number of attempts to be made to create a stream to a remote node over a direct unicast (1:1) connection before we give up.
	maxStreamCreationAttemptTimes uint64
}

// NewUnicastManager creates a new unicast manager.
// Args:
//   - cfg: configuration for the unicast manager.
//
// Returns:
//   - a new unicast manager.
//   - an error if the configuration is invalid, any error is irrecoverable.
func NewUnicastManager(cfg *ManagerConfig) (*Manager, error) {
	if cfg.MaxStreamCreationRetryAttemptTimes == uint64(0) {
		return nil, fmt.Errorf("max stream creation attempt times must be greater than 0")
	}
	if cfg.MaxDialRetryAttemptTimes == uint64(0) {
		return nil, fmt.Errorf("max dial attempt times must be greater than 0")
	}
	if cfg.DialZeroRetryResetThreshold == time.Duration(0) {
		return nil, fmt.Errorf("dial zero backoff reset threshold must be greater than 0")
	}
	if cfg.StreamZeroRetryResetThreshold == uint64(0) {
		return nil, fmt.Errorf("stream zero backoff reset threshold must be greater than 0")
	}
	if cfg.CreateStreamRetryDelay == time.Duration(0) {
		return nil, fmt.Errorf("create stream retry delay must be greater than 0")
	}

	return &Manager{
		logger: cfg.Logger.With().Str("module", "unicast-manager").Logger(),
		dialConfigCache: cfg.DialConfigCacheFactory(func() unicastmodel.DialConfig {
			return unicastmodel.DialConfig{
				StreamCreationRetryAttemptBudget: cfg.MaxStreamCreationRetryAttemptTimes,
				DialRetryAttemptBudget:           cfg.MaxDialRetryAttemptTimes,
			}
		}),
		streamFactory:                   cfg.StreamFactory,
		sporkId:                         cfg.SporkId,
		connStatus:                      cfg.ConnStatus,
		peerDialing:                     sync.Map{},
		metrics:                         cfg.Metrics,
		createStreamRetryDelay:          cfg.CreateStreamRetryDelay,
		streamZeroBackoffResetThreshold: cfg.StreamZeroRetryResetThreshold,
		dialZeroBackoffResetThreshold:   cfg.DialZeroRetryResetThreshold,
		maxStreamCreationAttemptTimes:   cfg.MaxStreamCreationRetryAttemptTimes,
		maxDialAttemptTimes:             cfg.MaxDialRetryAttemptTimes,
	}, nil
}

// SetDefaultHandler sets the default stream handler for this unicast manager. The default handler is utilized
// as the core handler for other unicast protocols, e.g., compressions.
func (m *Manager) SetDefaultHandler(defaultHandler libp2pnet.StreamHandler) {
	defaultProtocolID := protocols.FlowProtocolID(m.sporkId)
	if len(m.protocols) > 0 {
		panic("default handler must be set only once before any unicast registration")
	}

	m.defaultHandler = defaultHandler

	m.protocols = []protocols.Protocol{
		stream.NewPlainStream(defaultHandler, defaultProtocolID),
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
// it either creates a successful stream or runs out of options.
// Args:
//   - ctx: context for the stream creation.
//   - peerID: peer ID of the remote peer.
//
// Returns:
//   - a new libp2p stream.
//   - error if the stream creation fails; the error is benign and can be retried.
func (m *Manager) CreateStream(ctx context.Context, peerID peer.ID) (libp2pnet.Stream, error) {
	var errs error
	dialCfg, err := m.getDialConfig(peerID)
	if err != nil {
		// TODO: technically, we better to return an error here, but the error must be irrecoverable, and we cannot
		//       guarantee a clear distinction between recoverable and irrecoverable errors at the moment with CreateStream.
		//       We have to revisit this once we studied the error handling paths in the unicast manager.
		m.logger.Fatal().
			Err(err).
			Bool(logging.KeyNetworkingSecurity, true).
			Str("peer_id", p2plogging.PeerId(peerID)).
			Msg("failed to retrieve dial config for peer id")
	}

	m.logger.Debug().
		Str("peer_id", p2plogging.PeerId(peerID)).
		Str("dial_config", fmt.Sprintf("%+v", dialCfg)).
		Msg("dial config for the peer retrieved")

	for i := len(m.protocols) - 1; i >= 0; i-- {
		s, err := m.tryCreateStream(ctx, peerID, m.protocols[i], dialCfg)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		// return first successful stream
		return s, nil
	}

	connected, connErr := m.connStatus.IsConnected(peerID) // we don't check connErr as it indicates that the peer is not connected.
	updatedCfg, err := m.adjustUnsuccessfulStreamAttempt(peerID, connErr == nil && connected)
	if err != nil {
		// TODO: technically, we better to return an error here, but the error must be irrecoverable, and we cannot
		//       guarantee a clear distinction between recoverable and irrecoverable errors at the moment with CreateStream.
		//       We have to revisit this once we studied the error handling paths in the unicast manager.
		m.logger.Fatal().
			Err(err).
			Bool(logging.KeyNetworkingSecurity, true).
			Str("peer_id", p2plogging.PeerId(peerID)).
			Msg("failed to adjust dial config for peer id")
	}

	m.logger.Warn().
		Err(errs).
		Bool(logging.KeySuspicious, true).
		Str("peer_id", p2plogging.PeerId(peerID)).
		Str("dial_config", fmt.Sprintf("%+v", updatedCfg)).
		Bool("is_connected", err == nil && connected).
		Msg("failed to create stream to peer id, dial config adjusted")

	return nil, fmt.Errorf("could not create stream on any available unicast protocol: %w", errs)
}

// tryCreateStream will retry createStream with the configured exponential backoff delay and maxAttempts.
// During retries, each error encountered is aggregated in a multierror. If max attempts are made before a
// stream can be successfully the multierror will be returned. During stream creation when IsErrDialInProgress
// is encountered during retries this would indicate that no connection to the peer exists yet.
// In this case we will retry creating the stream with a backoff until a connection is established.
func (m *Manager) tryCreateStream(ctx context.Context, peerID peer.ID, protocol protocols.Protocol, dialCfg *unicastmodel.DialConfig) (libp2pnet.Stream, error) {
	var err error
	var s libp2pnet.Stream

	// configure back off retry delay values
	backoff := retry.NewExponential(m.createStreamRetryDelay)
	// https://github.com/sethvargo/go-retry#maxretries retries counter starts at zero and library will make last attempt
	// when retries == maxAttempts causing 1 more func invocation than expected.
	maxRetries := dialCfg.StreamCreationRetryAttemptBudget - 1
	if maxRetries < 0 {
		maxRetries = 0
	}
	backoff = retry.WithMaxRetries(maxRetries, backoff)

	attempts := 0
	// retryable func will attempt to create the stream and only retry if dialing the peer is in progress
	f := func(context.Context) error {
		attempts++
		s, err = m.createStream(ctx, peerID, protocol, dialCfg)
		if err != nil {
			if unicast.IsErrDialInProgress(err) {
				m.logger.Warn().
					Err(err).
					Str("peer_id", p2plogging.PeerId(peerID)).
					Int("attempt", attempts).
					Uint64("max_attempts", maxRetries).
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
		return nil, err
	}

	m.metrics.OnStreamCreated(duration, attempts)
	updatedConfig, err := m.dialConfigCache.Adjust(peerID, func(config unicastmodel.DialConfig) (unicastmodel.DialConfig, error) {
		config.ConsecutiveSuccessfulStream++ // increase consecutive successful stream count.
		return config, nil
	})
	if err != nil {
		// This is not a connection retryable error, this is a fatal error.
		// TODO: technically, we better to return an error here, but the error must be irrecoverable, and we cannot
		//       guarantee a clear distinction between recoverable and irrecoverable errors at the moment with CreateStream.
		//       We have to revisit this once we studied the error handling paths in the unicast manager.
		m.logger.Fatal().
			Err(err).
			Bool(logging.KeyNetworkingSecurity, true).
			Str("peer_id", p2plogging.PeerId(peerID)).
			Msg("failed to adjust dial config for peer id")
	}
	m.logger.Debug().
		Str("peer_id", p2plogging.PeerId(peerID)).
		Str("updated_dial_config", fmt.Sprintf("%+v", updatedConfig)).
		Msg("stream created successfully")
	return s, nil
}

// createStream creates a stream to the peerID with the provided protocol.
func (m *Manager) createStream(ctx context.Context, peerID peer.ID, protocol protocols.Protocol, dialCfg *unicastmodel.DialConfig) (libp2pnet.Stream, error) {
	s, err := m.rawStreamWithProtocol(ctx, protocol.ProtocolId(), peerID, dialCfg)
	if err != nil {
		return nil, err
	}

	s, err = protocol.UpgradeRawStream(s)
	if err != nil {
		return nil, err
	}

	return s, nil
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
func (m *Manager) rawStreamWithProtocol(ctx context.Context, protocolID protocol.ID, peerID peer.ID, dialCfg *unicastmodel.DialConfig) (libp2pnet.Stream, error) {
	isConnected, err := m.connStatus.IsConnected(peerID)
	if err != nil {
		return nil, err
	}

	// check connection status and attempt to dial the peer if dialing is not in progress
	if !isConnected {
		// return error if we can't start dialing
		if m.dialingInProgress(peerID) {
			return nil, unicast.NewDialInProgressErr(peerID)
		}
		defer m.dialingComplete(peerID)
		err := m.dialPeer(ctx, peerID, dialCfg)
		if err != nil {
			return nil, err
		}
	}

	// at this point dialing should have completed, we are already connected we can attempt to create the stream
	s, err := m.rawStream(ctx, peerID, protocolID, dialCfg)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// dialPeer dial peer with retries.
// Expected errors during normal operations:
//   - ErrMaxRetries if retry attempts are exhausted
func (m *Manager) dialPeer(ctx context.Context, peerID peer.ID, dialCfg *unicastmodel.DialConfig) error {
	// aggregated retryable errors that occur during retries, errs will be returned
	// if retry context times out or maxAttempts have been made before a successful retry occurs
	var errs error
	dialAttempts := 0
	backoff := retryBackoff(dialCfg.DialRetryAttemptBudget)
	f := func(context.Context) error {
		dialAttempts++
		select {
		case <-ctx.Done():
			return fmt.Errorf("context done before stream could be created (retry attempt: %d, errors: %w)", dialAttempts, errs)
		default:
		}
		err := m.streamFactory.Connect(ctx, peer.AddrInfo{ID: peerID})
		if err != nil {
			// if the connection was rejected due to invalid node id or
			// if the connection was rejected due to connection gating skip the re-attempt
			if stream.IsErrSecurityProtocolNegotiationFailed(err) || stream.IsErrGaterDisallowedConnection(err) {
				return multierror.Append(errs, err)
			}
			m.logger.Warn().
				Err(err).
				Str("peer_id", p2plogging.PeerId(peerID)).
				Int("attempt", dialAttempts).
				Uint64("max_attempts", dialCfg.DialRetryAttemptBudget).
				Msg("retrying peer dialing")
			return retry.RetryableError(multierror.Append(errs, err))
		}
		updatedConfig, err := m.dialConfigCache.Adjust(peerID, func(config unicastmodel.DialConfig) (unicastmodel.DialConfig, error) {
			config.LastSuccessfulDial = time.Now() // update last successful dial time
			return config, nil
		})
		if err != nil {
			// This is not a connection retryable error, this is a fatal error.
			// TODO: technically, we better to return an error here, but the error must be irrecoverable, and we cannot
			//       guarantee a clear distinction between recoverable and irrecoverable errors at the moment with CreateStream.
			//       We have to revisit this once we studied the error handling paths in the unicast manager.
			m.logger.Fatal().
				Err(err).
				Bool(logging.KeyNetworkingSecurity, true).
				Str("peer_id", p2plogging.PeerId(peerID)).
				Msg("failed to adjust dial config for peer id")
		}
		m.logger.Info().
			Str("peer_id", p2plogging.PeerId(peerID)).
			Str("updated_dial_config", fmt.Sprintf("%+v", updatedConfig)).
			Msg("peer dialed successfully")
		return nil
	}

	start := time.Now()
	err := retry.Do(ctx, backoff, f)
	duration := time.Since(start)
	if err != nil {
		m.metrics.OnPeerDialFailure(duration, dialAttempts)
		return retryFailedError(uint64(dialAttempts), dialCfg.DialRetryAttemptBudget, fmt.Errorf("failed to dial peer %s: %w", p2plogging.PeerId(peerID), err))
	}
	m.metrics.OnPeerDialed(duration, dialAttempts)
	return nil
}

// rawStream creates a stream to peer with retries.
// Expected errors during normal operations:
//   - ErrMaxRetries if retry attempts are exhausted
func (m *Manager) rawStream(ctx context.Context, peerID peer.ID, protocolID protocol.ID, dialCfg *unicastmodel.DialConfig) (libp2pnet.Stream, error) {
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
	err := retry.Do(ctx, retryBackoff(dialCfg.StreamCreationRetryAttemptBudget), f)
	duration := time.Since(start)
	if err != nil {
		m.metrics.OnEstablishStreamFailure(duration, attempts)
		return nil, retryFailedError(uint64(attempts), dialCfg.StreamCreationRetryAttemptBudget, fmt.Errorf("failed to create a stream to peer: %w", err))
	}
	m.metrics.OnStreamEstablished(duration, attempts)
	return s, nil
}

// retryBackoff returns an exponential retry with jitter and max attempts.
func retryBackoff(maxAttempts uint64) retry.Backoff {
	// create backoff
	backoff := retry.NewConstant(time.Second)
	// add a MaxRetryJitter*time.Millisecond jitter to our backoff to ensure that this node and the target node don't attempt to reconnect at the same time
	backoff = retry.WithJitter(MaxRetryJitter*time.Millisecond, backoff)
	maxRetries := maxAttempts
	if maxAttempts >= 0 {
		// https://github.com/sethvargo/go-retry#maxretries retries counter starts at zero and library will make last attempt
		// when retries == maxAttempts causing 1 more func invocation than expected.
		maxRetries = maxAttempts - 1
	}
	backoff = retry.WithMaxRetries(maxRetries, backoff)
	return backoff
}

// retryFailedError wraps the given error in a ErrMaxRetries if maxAttempts were made.
func retryFailedError(dialAttempts, maxAttempts uint64, err error) error {
	if dialAttempts == maxAttempts {
		return unicast.NewMaxRetriesErr(dialAttempts, err)
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

// getDialConfig gets the dial config for the given peer id.
// It also adjusts the dial config if necessary based on the current dial config, i.e., it resets the dial backoff budget to the default value if the last successful dial was long enough ago,
// and it resets the stream creation backoff budget to the default value if the number of consecutive successful streams reaches the threshold.
// Args:
//   - peerID: peer id of the remote peer.
//
// Returns:
//   - dial config for the given peer id.
//   - error if the dial config cannot be retrieved or adjusted; any error is irrecoverable and indicates a fatal error.
func (m *Manager) getDialConfig(peerID peer.ID) (*unicastmodel.DialConfig, error) {
	dialCfg, err := m.dialConfigCache.GetOrInit(peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get or init dial config for peer id: %w", err)
	}

	if dialCfg.StreamCreationRetryAttemptBudget == uint64(0) && dialCfg.ConsecutiveSuccessfulStream >= m.streamZeroBackoffResetThreshold {
		// reset the stream creation backoff budget to the default value if the number of consecutive successful streams reaches the threshold,
		// as the stream creation is reliable enough to be trusted again.
		dialCfg, err = m.dialConfigCache.Adjust(peerID, func(config unicastmodel.DialConfig) (unicastmodel.DialConfig, error) {
			config.StreamCreationRetryAttemptBudget = m.maxStreamCreationAttemptTimes
			return config, nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to adjust dial config for peer id (resetting stream creation attempt budget): %w", err)
		}
	}
	if dialCfg.DialRetryAttemptBudget == uint64(0) &&
		!dialCfg.LastSuccessfulDial.IsZero() && // if the last successful dial time is zero, it means that we have never successfully dialed to the peer, so we should not reset the dial backoff budget.
		time.Since(dialCfg.LastSuccessfulDial) >= m.dialZeroBackoffResetThreshold {
		// reset the dial backoff budget to the default value if the last successful dial was long enough ago,
		// as the dialing is reliable enough to be trusted again.
		dialCfg, err = m.dialConfigCache.Adjust(peerID, func(config unicastmodel.DialConfig) (unicastmodel.DialConfig, error) {
			config.DialRetryAttemptBudget = m.maxDialAttemptTimes
			return config, nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to adjust dial config for peer id (resetting dial attempt budget): %w", err)
		}
	}
	return dialCfg, nil
}

// adjustUnsuccessfulStreamAttempt adjusts the dial config for the given peer id if the stream creation fails.
// It resets the stream creation backoff budget to the default value if the number of consecutive successful streams reaches the threshold,
// and it resets the dial backoff budget to the default value if there is no connection to the peer.
// Args:
//   - peerID: peer id of the remote peer.
//
// Returns:
// - dial config for the given peer id.
// - connected indicates whether there is a connection to the peer.
// - error if the dial config cannot be adjusted; any error is irrecoverable and indicates a fatal error.
func (m *Manager) adjustUnsuccessfulStreamAttempt(peerID peer.ID, connected bool) (*unicastmodel.DialConfig, error) {
	updatedCfg, err := m.dialConfigCache.Adjust(peerID, func(config unicastmodel.DialConfig) (unicastmodel.DialConfig, error) {
		// consecutive successful stream count is reset to 0 if we fail to create a stream or connection to the peer.
		config.ConsecutiveSuccessfulStream = 0

		if !connected {
			// if no connections could be established to the peer, we will try to dial with a more strict dial config next time.
			if config.DialRetryAttemptBudget > 0 {
				config.DialRetryAttemptBudget--
			}
			// last successful dial time is reset to 0 if we fail to create a stream to the peer.
			config.LastSuccessfulDial = time.Time{}

		} else {
			// there is a connection to the peer it means that the stream creation failed, hence we decrease the stream backoff budget
			// to try to create a stream with a more strict dial config next time.
			if config.StreamCreationRetryAttemptBudget > 0 {
				config.StreamCreationRetryAttemptBudget--
			}
		}
		return config, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to adjust dial config for peer id: %w", err)
	}

	return updatedCfg, nil
}
