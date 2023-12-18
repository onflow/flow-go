package unicast

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// MaxRetryJitter is the maximum number of milliseconds to wait between attempts for a 1-1 direct connection
	MaxRetryJitter = 5
)

var (
	_ p2p.UnicastManager = (*Manager)(nil)
)

type DialConfigCacheFactory func(configFactory func() Config) ConfigCache

// Manager manages libp2p stream negotiation and creation, which is utilized for unicast dispatches.
type Manager struct {
	logger         zerolog.Logger
	streamFactory  p2p.StreamFactory
	protocols      []protocols.Protocol
	defaultHandler libp2pnet.StreamHandler
	sporkId        flow.Identifier
	metrics        module.UnicastManagerMetrics

	// createStreamBackoffDelay is the delay between each stream creation retry attempt.
	// The manager uses an exponential backoff strategy to retry stream creation, and this parameter
	// is the initial delay between each retry attempt. The delay is doubled after each retry attempt.
	createStreamBackoffDelay time.Duration

	// dialConfigCache is a cache to store the dial config for each peer.
	// TODO: encapsulation can be further improved by wrapping the dialConfigCache together with the dial config adjustment logic into a single struct.
	dialConfigCache ConfigCache

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
	if err := validator.New().Struct(cfg); err != nil {
		return nil, fmt.Errorf("invalid unicast manager config: %w", err)
	}

	m := &Manager{
		logger: cfg.Logger.With().Str("module", "unicast-manager").Logger(),
		dialConfigCache: cfg.UnicastConfigCacheFactory(func() Config {
			return Config{
				StreamCreationRetryAttemptBudget: cfg.Parameters.MaxStreamCreationRetryAttemptTimes,
			}
		}),
		streamFactory:                   cfg.StreamFactory,
		sporkId:                         cfg.SporkId,
		metrics:                         cfg.Metrics,
		createStreamBackoffDelay:        cfg.Parameters.CreateStreamBackoffDelay,
		streamZeroBackoffResetThreshold: cfg.Parameters.StreamZeroRetryResetThreshold,
		maxStreamCreationAttemptTimes:   cfg.Parameters.MaxStreamCreationRetryAttemptTimes,
	}

	m.logger.Info().
		Hex("spork_id", logging.ID(cfg.SporkId)).
		Dur("create_stream_backoff_delay", cfg.Parameters.CreateStreamBackoffDelay).
		Uint64("stream_zero_backoff_reset_threshold", cfg.Parameters.StreamZeroRetryResetThreshold).
		Msg("unicast manager created")

	return m, nil
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
		s, err := m.createStream(ctx, peerID, m.protocols[i], dialCfg)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		// return first successful stream
		return s, nil
	}

	updatedCfg, err := m.adjustUnsuccessfulStreamAttempt(peerID)
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
		Msg("failed to create stream to peer id, dial config adjusted")

	return nil, fmt.Errorf("could not create stream on any available unicast protocol: %w", errs)
}

// createStream attempts to establish a new stream with a peer using the specified protocol. It employs
// exponential backoff with a maximum number of attempts defined by dialCfg.StreamCreationRetryAttemptBudget.
// If the stream cannot be established after the maximum attempts, it returns a compiled multierror of all
// encountered errors. Errors related to in-progress dials trigger a retry until a connection is established
// or the attempt budget is exhausted.
//
// The function increments the Config's ConsecutiveSuccessfulStream count upon success. In the case of
// adjustment errors in Config, a fatal error is logged indicating an issue that requires attention.
// Metrics are collected to monitor the duration and number of attempts for stream creation.
//
// Arguments:
// - ctx: Context to control the lifecycle of the stream creation.
// - peerID: The ID of the peer with which the stream is to be established.
// - protocol: The specific protocol used for the stream.
// - dialCfg: Configuration parameters for dialing and stream creation, including retry logic.
//
// Returns:
// - libp2pnet.Stream: The successfully created stream, or nil if the stream creation fails.
// - error: An aggregated multierror of all encountered errors during stream creation, or nil if successful; any returned error is benign and can be retried.
func (m *Manager) createStream(ctx context.Context, peerID peer.ID, protocol protocols.Protocol, dialCfg *Config) (libp2pnet.Stream, error) {
	var err error
	var s libp2pnet.Stream

	s, err = m.createStreamWithRetry(ctx, peerID, protocol.ProtocolId(), dialCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create a stream to peer: %w", err)
	}

	s, err = protocol.UpgradeRawStream(s)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade raw stream: %w", err)
	}

	updatedConfig, err := m.dialConfigCache.Adjust(peerID, func(config Config) (Config, error) {
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

// createStreamWithRetry attempts to create a new stream to the specified peer using the given protocolID.
// This function is streamlined for use-cases where retries are managed externally or
// not required at all.
//
// Expected errors:
//   - If the context expires before stream creation, it returns a context-related error with the number of attempts.
//   - If the protocol ID is not supported, no retries are attempted and the error is returned immediately.
//
// Metrics are collected to monitor the duration and attempts of the stream creation process.
//
// Arguments:
// - ctx: Context to control the lifecycle of the stream creation.
// - peerID: The ID of the peer with which the stream is to be established.
// - protocolID: The identifier for the protocol used for the stream.
// - dialCfg: Configuration parameters for dialing, including the retry attempt budget.
//
// Returns:
// - libp2pnet.Stream: The successfully created stream, or nil if an error occurs.
// - error: An error encountered during the stream creation, or nil if the stream is successfully established.
func (m *Manager) createStreamWithRetry(ctx context.Context, peerID peer.ID, protocolID protocol.ID, dialCfg *Config) (libp2pnet.Stream, error) {
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
		// creates stream using stream factory
		s, err = m.streamFactory.NewStream(ctx, peerID, protocolID)
		if err != nil {
			// if the stream creation failed due to invalid protocol id or no address, skip the re-attempt
			if stream.IsErrProtocolNotSupported(err) ||
				errors.Is(err, swarm.ErrNoAddresses) ||
				stream.IsErrSecurityProtocolNegotiationFailed(err) ||
				stream.IsErrGaterDisallowedConnection(err) {
				return err
			}
			return retry.RetryableError(multierror.Append(errs, err))
		}
		return nil
	}

	start := time.Now()
	err := retry.Do(ctx, retryBackoff(dialCfg.StreamCreationRetryAttemptBudget, m.createStreamBackoffDelay), f)
	duration := time.Since(start)
	if err != nil {
		m.metrics.OnEstablishStreamFailure(duration, attempts)
		return nil, retryFailedError(uint64(attempts), dialCfg.StreamCreationRetryAttemptBudget, fmt.Errorf("failed to create a stream to peer: %w", err))
	}
	m.metrics.OnStreamEstablished(duration, attempts)
	return s, nil
}

// retryBackoff creates and returns a retry exponential backoff with the given maximum number of retries.
// Note that the retryBackoff by default makes one attempt. Hence, that total number of attempts are 1 + maxRetries.
// Args:
// - maxRetries: maximum number of retries (in addition to the first backoff).
// - retryInterval: initial retry interval for exponential backoff.
// Returns:
// - a retry backoff object that makes maximum of maxRetries + 1 attempts.
func retryBackoff(maxRetries uint64, retryInterval time.Duration) retry.Backoff {
	// create backoff
	backoff := retry.NewConstant(retryInterval)
	// add a MaxRetryJitter*time.Millisecond jitter to our backoff to ensure that this node and the target node don't attempt to reconnect at the same time
	backoff = retry.WithJitter(MaxRetryJitter*time.Millisecond, backoff)

	// https://github.com/sethvargo/go-retry#maxretries retries counter starts at zero and library will make last attempt
	// when retries == maxRetries. Hence, the total number of invocations is maxRetires + 1
	backoff = retry.WithMaxRetries(maxRetries, backoff)
	return backoff
}

// retryFailedError wraps the given error in a ErrMaxRetries if maxAttempts were made.
func retryFailedError(dialAttempts, maxAttempts uint64, err error) error {
	if dialAttempts == maxAttempts {
		return NewMaxRetriesErr(dialAttempts, err)
	}
	return err
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
func (m *Manager) getDialConfig(peerID peer.ID) (*Config, error) {
	dialCfg, err := m.dialConfigCache.GetOrInit(peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get or init dial config for peer id: %w", err)
	}

	if dialCfg.StreamCreationRetryAttemptBudget == uint64(0) && dialCfg.ConsecutiveSuccessfulStream >= m.streamZeroBackoffResetThreshold {
		// reset the stream creation backoff budget to the default value if the number of consecutive successful streams reaches the threshold,
		// as the stream creation is reliable enough to be trusted again.
		dialCfg, err = m.dialConfigCache.Adjust(peerID, func(config Config) (Config, error) {
			config.StreamCreationRetryAttemptBudget = m.maxStreamCreationAttemptTimes
			m.metrics.OnStreamCreationRetryBudgetUpdated(config.StreamCreationRetryAttemptBudget)
			m.metrics.OnStreamCreationRetryBudgetResetToDefault()
			return config, nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to adjust dial config for peer id (resetting stream creation attempt budget): %w", err)
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
func (m *Manager) adjustUnsuccessfulStreamAttempt(peerID peer.ID) (*Config, error) {
	updatedCfg, err := m.dialConfigCache.Adjust(peerID, func(config Config) (Config, error) {
		// consecutive successful stream count is reset to 0 if we fail to create a stream or connection to the peer.
		config.ConsecutiveSuccessfulStream = 0

		// there is a connection to the peer it means that the stream creation failed, hence we decrease the stream backoff budget
		// to try to create a stream with a more strict dial config next time.
		if config.StreamCreationRetryAttemptBudget > 0 {
			config.StreamCreationRetryAttemptBudget--
			m.metrics.OnStreamCreationRetryBudgetUpdated(config.StreamCreationRetryAttemptBudget)
		}

		return config, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to adjust dial config for peer id: %w", err)
	}

	return updatedCfg, nil
}
