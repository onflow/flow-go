package manager

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

// MisbehaviorReportManager is responsible for handling misbehavior reports.
// The current version is at the minimum viable product stage and only logs the reports.
// TODO: the mature version should be able to handle the reports and take actions accordingly, i.e., penalize the misbehaving node
//
//	and report the node to be disallow-listed if the overall penalty of the misbehaving node drops below the disallow-listing threshold.
type MisbehaviorReportManager struct {
	logger  zerolog.Logger
	metrics module.AlspMetrics
	cache   alsp.SpamRecordCache
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
func NewMisbehaviorReportManager(cfg *MisbehaviorReportManagerConfig) *MisbehaviorReportManager {
	cache := internal.NewSpamRecordCache(cfg.SpamRecordsCacheSize, cfg.Logger, cfg.CacheMetrics, model.SpamRecordFactory())

	return &MisbehaviorReportManager{
		logger:  cfg.Logger.With().Str("module", "misbehavior_report_manager").Logger(),
		metrics: cfg.AlspMetrics,
		cache:   cache,
	}
}

// HandleMisbehaviorReport is called upon a new misbehavior is reported.
// The current version is at the minimum viable product stage and only logs the reports.
// The implementation of this function should be thread-safe and non-blocking.
// TODO: the mature version should be able to handle the reports and take actions accordingly, i.e., penalize the misbehaving node
//
//	and report the node to be disallow-listed if the overall penalty of the misbehaving node drops below the disallow-listing threshold.
func (m *MisbehaviorReportManager) HandleMisbehaviorReport(channel channels.Channel, report network.MisbehaviorReport) {
	m.metrics.OnMisbehaviorReported(channel.String(), report.Reason().String())

	m.logger.Debug().
		Str("channel", channel.String()).
		Hex("misbehaving_id", logging.ID(report.OriginId())).
		Str("reason", report.Reason().String()).
		Msg("received misbehavior report")

	//_ := func() (float64, error) {
	//	return m.cache.Adjust(report.OriginId(), func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
	//		record.Penalty -= report.Penalty()
	//		return record, nil
	//	})
	//}
}
