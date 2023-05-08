package alspmgr

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/network/alsp/internal"
	"github.com/onflow/flow-go/network/alsp/model"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	FatalMsgNegativePositivePenalty = "penalty value is positive, expected negative"
	FatalMsgFailedToApplyPenalty    = "failed to apply penalty to the spam record"
)

// MisbehaviorReportManager is responsible for handling misbehavior reports.
// The current version is at the minimum viable product stage and only logs the reports.
// TODO: the mature version should be able to handle the reports and take actions accordingly, i.e., penalize the misbehaving node
//
//	and report the node to be disallow-listed if the overall penalty of the misbehaving node drops below the disallow-listing threshold.
type MisbehaviorReportManager struct {
	logger  zerolog.Logger
	metrics module.AlspMetrics
	cache   alsp.SpamRecordCache
	// disablePenalty indicates whether applying the penalty to the misbehaving node is disabled.
	// When disabled, the ALSP module logs the misbehavior reports and updates the metrics, but does not apply the penalty.
	// This is useful for managing production incidents.
	// Note: under normal circumstances, the ALSP module should not be disabled.
	disablePenalty bool
}

var _ network.MisbehaviorReportManager = (*MisbehaviorReportManager)(nil)

type MisbehaviorReportManagerConfig struct {
	Logger zerolog.Logger
	// SpamRecordsCacheSize is the size of the spam record cache that stores the spam records for the authorized nodes.
	// It should be as big as the number of authorized nodes in Flow network.
	// Recommendation: for small network sizes 10 * number of authorized nodes to ensure that the cache can hold all the spam records of the authorized nodes.
	SpamRecordsCacheSize uint32
	// AlspMetrics is the metrics instance for the alsp module (collecting spam reports).
	AlspMetrics module.AlspMetrics
	// CacheMetrics is the metrics factory for the spam record cache.
	CacheMetrics module.HeroCacheMetrics
	// DisablePenalty indicates whether applying the penalty to the misbehaving node is disabled.
	// When disabled, the ALSP module logs the misbehavior reports and updates the metrics, but does not apply the penalty.
	// This is useful for managing production incidents.
	// Note: under normal circumstances, the ALSP module should not be disabled.
	DisablePenalty bool
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
//	a new instance of the MisbehaviorReportManager.
func NewMisbehaviorReportManager(cfg *MisbehaviorReportManagerConfig, opts ...MisbehaviorReportManagerOption) *MisbehaviorReportManager {

	m := &MisbehaviorReportManager{
		logger:         cfg.Logger.With().Str("module", "misbehavior_report_manager").Logger(),
		metrics:        cfg.AlspMetrics,
		disablePenalty: cfg.DisablePenalty,
	}

	if m.disablePenalty {
		// when the penalty is enabled, the ALSP module is disabled only if the spam record cache is not set.
		m.logger.Warn().Msg("penalty mechanism of alsp is disabled")
		return m
	}

	m.cache = internal.NewSpamRecordCache(cfg.SpamRecordsCacheSize, cfg.Logger, cfg.CacheMetrics, model.SpamRecordFactory())

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// HandleMisbehaviorReport is called upon a new misbehavior is reported.
// The current version is at the minimum viable product stage and only logs the reports.
// The implementation of this function should be thread-safe and non-blocking.
// TODO: the mature version should be able to handle the reports and take actions accordingly, i.e., penalize the misbehaving node
// and report the node to be disallow-listed if the overall penalty of the misbehaving node drops below the disallow-listing threshold.
func (m *MisbehaviorReportManager) HandleMisbehaviorReport(channel channels.Channel, report network.MisbehaviorReport) {
	lg := m.logger.With().
		Str("channel", channel.String()).
		Hex("misbehaving_id", logging.ID(report.OriginId())).
		Str("reason", report.Reason().String()).
		Float64("penalty", report.Penalty()).Logger()
	m.metrics.OnMisbehaviorReported(channel.String(), report.Reason().String())

	if m.disablePenalty {
		// when penalty mechanism disabled, the misbehavior is logged and metrics are updated,
		// but no further actions are taken.
		lg.Trace().Msg("discarding misbehavior report because ALSP module is disabled")
		return
	}

	applyPenalty := func() (float64, error) {
		return m.cache.Adjust(report.OriginId(), func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
			if report.Penalty() > 0 {
				// this should never happen, unless there is a bug in the misbehavior report handling logic.
				// we should crash the node in this case to prevent further misbehavior reports from being lost and fix the bug.
				// TODO: refactor to throwing error to the irrecoverable context.
				lg.Fatal().Float64("penalty", report.Penalty()).Msg(FatalMsgNegativePositivePenalty)
			}
			record.Penalty += report.Penalty() // penalty value is negative. We add it to the current penalty.
			return record, nil
		})
	}

	init := func() {
		initialized := m.cache.Init(report.OriginId())
		lg.Trace().Bool("initialized", initialized).Msg("initialized spam record")
	}

	// we first try to apply the penalty to the spam record, if it does not exist, cache returns ErrSpamRecordNotFound.
	// in this case, we initialize the spam record and try to apply the penalty again. We use an optimistic update by
	// first assuming that the spam record exists and then initializing it if it does not exist. In this way, we avoid
	// acquiring the lock twice per misbehavior report, reducing the contention on the lock and improving the performance.
	updatedPenalty, err := internal.TryWithRecoveryIfHitError(internal.ErrSpamRecordNotFound, applyPenalty, init)
	if err != nil {
		// this should never happen, unless there is a bug in the spam record cache implementation.
		// we should crash the node in this case to prevent further misbehavior reports from being lost and fix the bug.
		// TODO: refactor to throwing error to the irrecoverable context.
		lg.Fatal().Err(err).Msg(FatalMsgFailedToApplyPenalty)
		return
	}

	lg.Debug().Float64("updated_penalty", updatedPenalty).Msg("misbehavior report handled")
}
