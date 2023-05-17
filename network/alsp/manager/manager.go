package alspmgr

import (
	crand "crypto/rand"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/network/alsp/internal"
	"github.com/onflow/flow-go/network/alsp/model"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// defaultMisbehaviorReportManagerWorkers is the default number of workers in the worker pool.
	defaultMisbehaviorReportManagerWorkers = 2
	FatalMsgNegativePositivePenalty        = "penalty value is positive, expected negative %f"
)

var (
	// ErrSpamRecordCacheSizeNotSet is returned when the spam record cache size is not set, it is a fatal irrecoverable error,
	// and the ALSP module cannot be initialized.
	ErrSpamRecordCacheSizeNotSet = errors.New("spam record cache size is not set")
	// ErrSpamReportQueueSizeNotSet is returned when the spam report queue size is not set, it is a fatal irrecoverable error,
	// and the ALSP module cannot be initialized.
	ErrSpamReportQueueSizeNotSet = errors.New("spam report queue size is not set")
)

// MisbehaviorReportManager is responsible for handling misbehavior reports.
// The current version is at the minimum viable product stage and only logs the reports.
// TODO: the mature version should be able to handle the reports and take actions accordingly, i.e., penalize the misbehaving node
//
//	and report the node to be disallow-listed if the overall penalty of the misbehaving node drops below the disallow-listing threshold.
type MisbehaviorReportManager struct {
	component.Component
	logger  zerolog.Logger
	metrics module.AlspMetrics
	cache   alsp.SpamRecordCache
	// disablePenalty indicates whether applying the penalty to the misbehaving node is disabled.
	// When disabled, the ALSP module logs the misbehavior reports and updates the metrics, but does not apply the penalty.
	// This is useful for managing production incidents.
	// Note: under normal circumstances, the ALSP module should not be disabled.
	disablePenalty bool

	// workerPool is the worker pool for handling the misbehavior reports in a thread-safe and non-blocking manner.
	workerPool *worker.Pool[internal.ReportedMisbehaviorWork]
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
	NetworkType p2p.NetworkType
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
	return nil
}

type MisbehaviorReportManagerOption func(*MisbehaviorReportManager)

// WithSpamRecordsCache sets the spam record cache for the MisbehaviorReportManager.
// Args:
//
//	cache: the spam record cache instance.
//
// Returns:
//
//	a MisbehaviorReportManagerOption that sets the spam record cache for the MisbehaviorReportManager.
//
// Note: this option is used for testing purposes. The production version of the MisbehaviorReportManager should use the
//
//	NewSpamRecordCache function to create the spam record cache.
func WithSpamRecordsCache(cache alsp.SpamRecordCache) MisbehaviorReportManagerOption {
	return func(m *MisbehaviorReportManager) {
		m.cache = cache
	}
}

// NewMisbehaviorReportManager creates a new instance of the MisbehaviorReportManager.
// Args:
//
//	logger: the logger instance.
//	metrics: the metrics instance.
//	cache: the spam record cache instance.
//
// Returns:
//
//		A new instance of the MisbehaviorReportManager.
//	 An error if the config is invalid. The error is considered irrecoverable.
func NewMisbehaviorReportManager(cfg *MisbehaviorReportManagerConfig, opts ...MisbehaviorReportManagerOption) (*MisbehaviorReportManager, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration for MisbehaviorReportManager: %w", err)
	}

	lg := cfg.Logger.With().Str("module", "misbehavior_report_manager").Logger()
	m := &MisbehaviorReportManager{
		logger:         lg,
		metrics:        cfg.AlspMetrics,
		disablePenalty: cfg.DisablePenalty,
	}

	m.cache = internal.NewSpamRecordCache(
		cfg.SpamRecordCacheSize,
		lg.With().Str("component", "spam_record_cache").Logger(),
		metrics.ApplicationLayerSpamRecordCacheMetricFactory(cfg.HeroCacheMetricsFactory, cfg.NetworkType),
		model.SpamRecordFactory())

	store := queue.NewHeroStore(
		cfg.SpamReportQueueSize,
		lg.With().Str("component", "spam_record_queue").Logger(),
		metrics.ApplicationLayerSpamRecordQueueMetricsFactory(cfg.HeroCacheMetricsFactory))

	m.workerPool = worker.NewWorkerPoolBuilder[internal.ReportedMisbehaviorWork](
		cfg.Logger,
		store,
		m.processMisbehaviorReport).Build()

	for _, opt := range opts {
		opt(m)
	}

	builder := component.NewComponentManagerBuilder()
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
			return record, fmt.Errorf(FatalMsgNegativePositivePenalty, report.Penalty)
		}
		record.Penalty += report.Penalty // penalty value is negative. We add it to the current penalty.
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
