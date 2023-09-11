package alspmgr

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/network/alsp/internal"
	"github.com/onflow/flow-go/network/alsp/model"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// defaultMisbehaviorReportManagerWorkers is the default number of workers in the worker pool.
	defaultMisbehaviorReportManagerWorkers = 2
)

var (
	// ErrSpamRecordCacheSizeNotSet is returned when the spam record cache size is not set, it is a fatal irrecoverable error,
	// and the ALSP module cannot be initialized.
	ErrSpamRecordCacheSizeNotSet = errors.New("spam record cache size is not set")
	// ErrSpamReportQueueSizeNotSet is returned when the spam report queue size is not set, it is a fatal irrecoverable error,
	// and the ALSP module cannot be initialized.
	ErrSpamReportQueueSizeNotSet = errors.New("spam report queue size is not set")
	// ErrHeartBeatIntervalNotSet is returned when the heartbeat interval is not set, it is a fatal irrecoverable error,
	// and the ALSP module cannot be initialized.
	ErrHeartBeatIntervalNotSet = errors.New("heartbeat interval is not set")
)

type SpamRecordCacheFactory func(zerolog.Logger, uint32, module.HeroCacheMetrics) alsp.SpamRecordCache

// SpamRecordDecayFunc is the function that calculates the decay of the spam record.
type SpamRecordDecayFunc func(model.ProtocolSpamRecord) float64

func defaultSpamRecordDecayFunc() SpamRecordDecayFunc {
	return func(record model.ProtocolSpamRecord) float64 {
		return math.Min(record.Penalty+record.Decay, 0)
	}
}

// defaultSpamRecordCacheFactory is the default spam record cache factory. It creates a new spam record cache with the given parameter.
func defaultSpamRecordCacheFactory() SpamRecordCacheFactory {
	return func(logger zerolog.Logger, size uint32, cacheMetrics module.HeroCacheMetrics) alsp.SpamRecordCache {
		return internal.NewSpamRecordCache(
			size,
			logger.With().Str("component", "spam_record_cache").Logger(),
			cacheMetrics,
			model.SpamRecordFactory())
	}
}

// MisbehaviorReportManager is responsible for handling misbehavior reports, i.e., penalizing the misbehaving node
// and report the node to be disallow-listed if the overall penalty of the misbehaving node drops below the disallow-listing threshold.
type MisbehaviorReportManager struct {
	component.Component
	logger  zerolog.Logger
	metrics module.AlspMetrics
	// cacheFactory is the factory for creating the spam record cache. MisbehaviorReportManager is coming with a
	// default factory that creates a new spam record cache with the given parameter. However, this factory can be
	// overridden with a custom factory.
	cacheFactory SpamRecordCacheFactory
	// cache is the spam record cache that stores the spam records for the authorized nodes. It is initialized by
	// invoking the cacheFactory.
	cache alsp.SpamRecordCache
	// disablePenalty indicates whether applying the penalty to the misbehaving node is disabled.
	// When disabled, the ALSP module logs the misbehavior reports and updates the metrics, but does not apply the penalty.
	// This is useful for managing production incidents.
	// Note: under normal circumstances, the ALSP module should not be disabled.
	disablePenalty bool

	// disallowListingConsumer is the consumer for the disallow-listing notifications.
	// It is notified when a node is disallow-listed by this manager.
	disallowListingConsumer network.DisallowListNotificationConsumer

	// workerPool is the worker pool for handling the misbehavior reports in a thread-safe and non-blocking manner.
	workerPool *worker.Pool[internal.ReportedMisbehaviorWork]

	// decayFunc is the function that calculates the decay of the spam record.
	decayFunc SpamRecordDecayFunc
}

var _ network.MisbehaviorReportManager = (*MisbehaviorReportManager)(nil)

type MisbehaviorReportManagerConfig struct {
	Logger zerolog.Logger
	// SpamRecordCacheSize is the size of the spam record cache that stores the spam records for the authorized nodes.
	// It should be as big as the number of authorized nodes in Flow network.
	// Recommendation: for small network sizes 10 * number of authorized nodes to ensure that the cache can hold all the spam records of the authorized nodes.
	SpamRecordCacheSize uint32
	// SpamReportQueueSize is the size of the queue that stores the spam records to be processed by the worker pool.
	SpamReportQueueSize uint32
	// AlspMetrics is the metrics instance for the alsp module (collecting spam reports).
	AlspMetrics module.AlspMetrics
	// HeroCacheMetricsFactory is the metrics factory for the HeroCache-related metrics.
	// Having factory as part of the config allows to create the metrics locally in the module.
	HeroCacheMetricsFactory metrics.HeroCacheMetricsFactory
	// DisablePenalty indicates whether applying the penalty to the misbehaving node is disabled.
	// When disabled, the ALSP module logs the misbehavior reports and updates the metrics, but does not apply the penalty.
	// This is useful for managing production incidents.
	// Note: under normal circumstances, the ALSP module should not be disabled.
	DisablePenalty bool
	// NetworkType is the type of the network it is used to determine whether the ALSP module is utilized in the
	// public (unstaked) or private (staked) network.
	NetworkType network.NetworkingType
	// HeartBeatInterval is the interval between the heartbeats. Heartbeat is a recurring event that is used to
	// apply recurring actions, e.g., decay the penalty of the misbehaving nodes.
	HeartBeatInterval time.Duration
	Opts              []MisbehaviorReportManagerOption
}

// validate validates the MisbehaviorReportManagerConfig instance. It returns an error if the config is invalid.
// It only validates the numeric fields of the config that may yield a stealth error in the production.
// It does not validate the struct fields of the config against a nil value.
// Args:
//
//	None.
//
// Returns:
//
//	An error if the config is invalid.
func (c MisbehaviorReportManagerConfig) validate() error {
	if c.SpamRecordCacheSize == 0 {
		return ErrSpamRecordCacheSizeNotSet
	}
	if c.SpamReportQueueSize == 0 {
		return ErrSpamReportQueueSizeNotSet
	}
	if c.HeartBeatInterval == 0 {
		return ErrHeartBeatIntervalNotSet
	}
	return nil
}

type MisbehaviorReportManagerOption func(*MisbehaviorReportManager)

// NewMisbehaviorReportManager creates a new instance of the MisbehaviorReportManager.
// Args:
// cfg: the configuration for the MisbehaviorReportManager.
// consumer: the consumer for the disallow-listing notifications. When the manager decides to disallow-list a node, it notifies the consumer to
// perform the lower-level disallow-listing action at the networking layer.
// All connections to the disallow-listed node are closed and the node is removed from the overlay, and
// no further connections are established to the disallow-listed node, either inbound or outbound.
//
// Returns:
//
//		A new instance of the MisbehaviorReportManager.
//	 An error if the config is invalid. The error is considered irrecoverable.
func NewMisbehaviorReportManager(cfg *MisbehaviorReportManagerConfig, consumer network.DisallowListNotificationConsumer) (*MisbehaviorReportManager, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration for MisbehaviorReportManager: %w", err)
	}

	lg := cfg.Logger.With().Str("module", "misbehavior_report_manager").Logger()
	m := &MisbehaviorReportManager{
		logger:                  lg,
		metrics:                 cfg.AlspMetrics,
		disablePenalty:          cfg.DisablePenalty,
		disallowListingConsumer: consumer,
		cacheFactory:            defaultSpamRecordCacheFactory(),
		decayFunc:               defaultSpamRecordDecayFunc(),
	}

	store := queue.NewHeroStore(
		cfg.SpamReportQueueSize,
		lg.With().Str("component", "spam_record_queue").Logger(),
		metrics.ApplicationLayerSpamRecordQueueMetricsFactory(cfg.HeroCacheMetricsFactory, cfg.NetworkType))

	m.workerPool = worker.NewWorkerPoolBuilder[internal.ReportedMisbehaviorWork](
		cfg.Logger,
		store,
		m.processMisbehaviorReport).Build()

	for _, opt := range cfg.Opts {
		opt(m)
	}

	m.cache = m.cacheFactory(
		lg,
		cfg.SpamRecordCacheSize,
		metrics.ApplicationLayerSpamRecordCacheMetricFactory(cfg.HeroCacheMetricsFactory, cfg.NetworkType))

	builder := component.NewComponentManagerBuilder()
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()
		m.heartbeatLoop(ctx, cfg.HeartBeatInterval) // blocking call
	})
	for i := 0; i < defaultMisbehaviorReportManagerWorkers; i++ {
		builder.AddWorker(m.workerPool.WorkerLogic())
	}

	m.Component = builder.Build()

	if m.disablePenalty {
		m.logger.Warn().Msg("penalty mechanism of alsp is disabled")
	}
	return m, nil
}

// HandleMisbehaviorReport is called upon a new misbehavior is reported.
// The implementation of this function should be thread-safe and non-blocking.
// Args:
//
//	channel: the channel on which the misbehavior is reported.
//	report: the misbehavior report.
//
// Returns:
//
//	none.
func (m *MisbehaviorReportManager) HandleMisbehaviorReport(channel channels.Channel, report network.MisbehaviorReport) {
	lg := m.logger.With().
		Str("channel", channel.String()).
		Hex("misbehaving_id", logging.ID(report.OriginId())).
		Str("reason", report.Reason().String()).
		Float64("penalty", report.Penalty()).Logger()
	lg.Trace().Msg("received misbehavior report")
	m.metrics.OnMisbehaviorReported(channel.String(), report.Reason().String())

	nonce := [internal.NonceSize]byte{}
	nonceSize, err := crand.Read(nonce[:])
	if err != nil {
		// this should never happen, but if it does, we should not continue
		lg.Fatal().Err(err).Msg("failed to generate nonce")
		return
	}
	if nonceSize != internal.NonceSize {
		// this should never happen, but if it does, we should not continue
		lg.Fatal().Msgf("nonce size mismatch: expected %d, got %d", internal.NonceSize, nonceSize)
		return
	}

	if ok := m.workerPool.Submit(internal.ReportedMisbehaviorWork{
		Channel:  channel,
		OriginId: report.OriginId(),
		Reason:   report.Reason(),
		Penalty:  report.Penalty(),
		Nonce:    nonce,
	}); !ok {
		lg.Warn().Msg("discarding misbehavior report because either the queue is full or the misbehavior report is duplicate")
	}

	lg.Debug().Msg("misbehavior report submitted")
}

// heartbeatLoop starts the heartbeat ticks ticker to tick at the given intervals. It is a blocking function, and
// should be called in a separate goroutine. It returns when the context is canceled. Hearbeats are recurring events that
// are used to perform periodic tasks.
// Args:
//
//	ctx: the context.
//	interval: the interval between two ticks.
//
// Returns:
//
//	none.
func (m *MisbehaviorReportManager) heartbeatLoop(ctx irrecoverable.SignalerContext, interval time.Duration) {
	ticker := time.NewTicker(interval)
	m.logger.Info().Dur("interval", interval).Msg("starting heartbeat ticks")
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			m.logger.Debug().Msg("heartbeat ticks stopped")
			return
		case <-ticker.C:
			m.logger.Trace().Msg("new heartbeat ticked")
			if err := m.onHeartbeat(); err != nil {
				// any error returned from onHeartbeat is considered irrecoverable.
				ctx.Throw(fmt.Errorf("failed to perform heartbeat: %w", err))
			}
		}
	}
}

// onHeartbeat is called upon a heartbeatLoop. It encapsulates the recurring tasks that should be performed
// during a heartbeat, which currently includes decay of the spam records.
// Args:
//
//	none.
//
// Returns:
//
//		error: if an error occurs, it is returned. No error is expected during normal operation. Any returned error must
//	 be considered as irrecoverable.
func (m *MisbehaviorReportManager) onHeartbeat() error {
	allIds := m.cache.Identities()

	for _, id := range allIds {
		m.logger.Trace().Hex("identifier", logging.ID(id)).Msg("onHeartbeat - looping through spam records")
		penalty, err := m.cache.Adjust(id, func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
			if record.Penalty > 0 {
				// sanity check; this should never happen.
				return record, fmt.Errorf("illegal state: spam record %x has positive penalty %f", id, record.Penalty)
			}
			if record.Decay <= 0 {
				// sanity check; this should never happen.
				return record, fmt.Errorf("illegal state: spam record %x has non-positive decay %f", id, record.Decay)
			}

			// TODO: this can be done in batch but at this stage let's send individual notifications.
			//       (it requires enabling the batch mode end-to-end including the cache in network).
			// as long as record.Penalty is NOT below model.DisallowListingThreshold,
			// the node is considered allow-listed and can conduct inbound and outbound connections.
			// Once it falls below model.DisallowListingThreshold, it needs to be disallow listed.
			if record.Penalty < model.DisallowListingThreshold && !record.DisallowListed {
				// cutoff counter keeps track of how many times the penalty has been below the threshold.
				record.CutoffCounter++
				record.DisallowListed = true
				// Adjusts decay dynamically based on how many times the node was disallow-listed (cutoff).
				record.Decay = m.adjustDecayFunc(record.CutoffCounter)
				m.logger.Warn().
					Str("key", logging.KeySuspicious).
					Hex("identifier", logging.ID(id)).
					Float64("penalty", record.Penalty).
					Uint64("cutoff_counter", record.CutoffCounter).
					Float64("decay_speed", record.Decay).
					Bool("disallow_listed", record.DisallowListed).
					Msg("node penalty dropped below threshold, initiating disallow listing")
				m.disallowListingConsumer.OnDisallowListNotification(&network.DisallowListingUpdate{
					FlowIds: flow.IdentifierList{id},
					Cause:   network.DisallowListedCauseAlsp, // sets the ALSP disallow listing cause on node
				})
			}
			// each time we decay the penalty by the decay speed, the penalty is a negative number, and the decay speed
			// is a positive number. So the penalty is getting closer to zero.
			// We use math.Min() to make sure the penalty is never positive.
			m.logger.Trace().
				Hex("identifier", logging.ID(id)).
				Uint64("cutoff_counter", record.CutoffCounter).
				Bool("disallow_listed", record.DisallowListed).
				Float64("penalty", record.Penalty).
				Msg("heartbeat interval, pulled the spam record for decaying")
			record.Penalty = m.decayFunc(record)
			m.logger.Trace().
				Hex("identifier", logging.ID(id)).
				Uint64("cutoff_counter", record.CutoffCounter).
				Bool("disallow_listed", record.DisallowListed).
				Float64("penalty", record.Penalty).
				Msg("heartbeat interval, spam record penalty adjusted by decay function")

			// TODO: this can be done in batch but at this stage let's send individual notifications.
			//       (it requires enabling the batch mode end-to-end including the cache in network).
			if record.Penalty == float64(0) && record.DisallowListed {
				record.DisallowListed = false

				m.logger.Info().
					Hex("identifier", logging.ID(id)).
					Uint64("cutoff_counter", record.CutoffCounter).
					Float64("decay_speed", record.Decay).
					Bool("disallow_listed", record.DisallowListed).
					Msg("allow-listing a node that was disallow listed")
				// Penalty has fully decayed to zero and the node can be back in the allow list.
				m.disallowListingConsumer.OnAllowListNotification(&network.AllowListingUpdate{
					FlowIds: flow.IdentifierList{id},
					Cause:   network.DisallowListedCauseAlsp, // clears the ALSP disallow listing cause from node
				})
			}

			m.logger.Trace().
				Hex("identifier", logging.ID(id)).
				Uint64("cutoff_counter", record.CutoffCounter).
				Float64("decay_speed", record.Decay).
				Bool("disallow_listed", record.DisallowListed).
				Msg("spam record decayed successfully")
			return record, nil
		})

		// any error here is fatal because it indicates a bug in the cache. All ids being iterated over are in the cache,
		// and adjust function above should not return an error unless there is a bug.
		if err != nil {
			return fmt.Errorf("failed to decay spam record %x: %w", id, err)
		}

		m.logger.Trace().
			Hex("identifier", logging.ID(id)).
			Float64("updated_penalty", penalty).
			Msg("spam record decayed")
	}

	return nil
}

// processMisbehaviorReport is the worker function that processes the misbehavior reports.
// It is called by the worker pool.
// It applies the penalty to the misbehaving node and updates the spam record cache.
// Implementation must be thread-safe so that it can be called concurrently.
// Args:
//
//	report: the misbehavior report to be processed.
//
// Returns:
//
//		error: the error that occurred during the processing of the misbehavior report. The returned error is
//	 irrecoverable and the node should crash if it occurs (indicating a bug in the ALSP module).
func (m *MisbehaviorReportManager) processMisbehaviorReport(report internal.ReportedMisbehaviorWork) error {
	lg := m.logger.With().
		Str("channel", report.Channel.String()).
		Hex("misbehaving_id", logging.ID(report.OriginId)).
		Str("reason", report.Reason.String()).
		Float64("penalty", report.Penalty).Logger()

	if m.disablePenalty {
		// when penalty mechanism disabled, the misbehavior is logged and metrics are updated,
		// but no further actions are taken.
		lg.Trace().Msg("discarding misbehavior report because alsp penalty is disabled")
		return nil
	}

	// Adjust will first try to apply the penalty to the spam record, if it does not exist, the Adjust method will initialize
	// a spam record for the peer first and then applies the penalty. In other words, Adjust uses an optimistic update by
	// first assuming that the spam record exists and then initializing it if it does not exist. In this way, we avoid
	// acquiring the lock twice per misbehavior report, reducing the contention on the lock and improving the performance.
	updatedPenalty, err := m.cache.Adjust(report.OriginId, func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		if report.Penalty > 0 {
			// this should never happen, unless there is a bug in the misbehavior report handling logic.
			// we should crash the node in this case to prevent further misbehavior reports from being lost and fix the bug.
			// we return the error as it is considered as a fatal error.
			return record, fmt.Errorf("penalty value is positive, expected negative %f", report.Penalty)
		}
		record.Penalty += report.Penalty // penalty value is negative. We add it to the current penalty.
		lg = lg.With().
			Float64("penalty_before_update", record.Penalty).
			Uint64("cutoff_counter", record.CutoffCounter).
			Float64("decay_speed", record.Decay).
			Bool("disallow_listed", record.DisallowListed).
			Logger()
		return record, nil
	})
	if err != nil {
		// this should never happen, unless there is a bug in the spam record cache implementation.
		// we should crash the node in this case to prevent further misbehavior reports from being lost and fix the bug.
		return fmt.Errorf("failed to apply penalty to the spam record: %w", err)
	}
	lg.Debug().Float64("updated_penalty", updatedPenalty).Msg("misbehavior report handled")
	return nil
}

// adjustDecayFunc calculates the decay value of the spam record cache. This allows the decay to be different on subsequent disallow listings.
// It returns the decay speed for the given cutoff counter.
// The cutoff counter is the number of times that the node has been disallow-listed.
// Args:
// cutoffCounter: the number of times that the node has been disallow-listed including the current time. Note that the cutoff counter
// must always be updated before calling this function.
//
// Returns:
//
//	float64: the decay speed for the given cutoff counter.
func (m *MisbehaviorReportManager) adjustDecayFunc(cutoffCounter uint64) float64 {
	// decaySpeeds illustrates the decay speeds for different cutoff counters.
	// The first cutoff does not reduce the decay speed (1000 -> 1000). However, the second, third,
	// and forth cutoffs reduce the decay speed by 90% (1000 -> 100, 100 -> 10, 10 -> 1).
	// All subsequent cutoffs after the fourth cutoff use the last decay speed (1).
	// This is to prevent the decay speed from becoming too small and the spam record from taking too long to decay.
	switch {
	case cutoffCounter == 1:
		return 1000
	case cutoffCounter == 2:
		return 100
	case cutoffCounter == 3:
		return 10
	case cutoffCounter >= 4:
		return 1
	default:
		panic(fmt.Sprintf("illegal-state cutoff counter must be positive, it should include the current time: %d", cutoffCounter))
	}
}

// WithSpamRecordsCacheFactory sets the spam record cache factory for the MisbehaviorReportManager.
// Args:
//
//	f: the spam record cache factory.
//
// Returns:
//
//	a MisbehaviorReportManagerOption that sets the spam record cache for the MisbehaviorReportManager.
//
// Note: this option is useful primarily for testing purposes. The default factory should be sufficient for production.
func WithSpamRecordsCacheFactory(f SpamRecordCacheFactory) MisbehaviorReportManagerOption {
	return func(m *MisbehaviorReportManager) {
		m.cacheFactory = f
	}
}

// WithDecayFunc sets the decay function for the MisbehaviorReportManager. Useful for testing purposes to simulate the decay of the penalty without waiting for the actual decay.
// Args:
//
//	f: the decay function.
//
// Returns:
//
//	a MisbehaviorReportManagerOption that sets the decay function for the MisbehaviorReportManager.
//
// Note: this option is useful primarily for testing purposes. The default decay function should be used for production.
func WithDecayFunc(f SpamRecordDecayFunc) MisbehaviorReportManagerOption {
	return func(m *MisbehaviorReportManager) {
		m.decayFunc = f
	}
}
