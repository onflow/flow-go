package alsp

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
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
}

var _ network.MisbehaviorReportManager = (*MisbehaviorReportManager)(nil)

// NewMisbehaviorReportManager creates a new instance of the MisbehaviorReportManager.
func NewMisbehaviorReportManager(logger zerolog.Logger, metrics module.AlspMetrics) *MisbehaviorReportManager {
	return &MisbehaviorReportManager{
		logger:  logger.With().Str("module", "misbehavior_report_manager").Logger(),
		metrics: metrics,
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

	// TODO: handle the misbehavior report and take actions accordingly.
}
