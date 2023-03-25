package alsp

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/logging"
)

// MisbehaviorReportManager is responsible for handling misbehavior reports.
// The current version is at the minimum viable product stage and only logs the reports.
// TODO: the mature version should be able to handle the reports and take actions accordingly, i.e., penalize the misbehaving node
//       and report the node to be disallow-listed if the overall penalty of the misbehaving node drops below the disallow-listing threshold.
type MisbehaviorReportManager struct {
	logger zerolog.Logger
}

var _ network.MisbehaviorReportManager = (*MisbehaviorReportManager)(nil)

// NewMisbehaviorReportManager creates a new instance of the MisbehaviorReportManager.
func NewMisbehaviorReportManager(logger zerolog.Logger) *MisbehaviorReportManager {
	return &MisbehaviorReportManager{
		logger: logger.With().Str("module", "missbehavior_report_manager").Logger(),
	}
}

// HandleReportedMisbehavior is called upon a new misbehavior is reported.
// The current version is at the minimum viable product stage and only logs the reports.
// The implementation of this function should be thread-safe and non-blocking.
// TODO: the mature version should be able to handle the reports and take actions accordingly, i.e., penalize the misbehaving node
//       and report the node to be disallow-listed if the overall penalty of the misbehaving node drops below the disallow-listing threshold.
func (m MisbehaviorReportManager) HandleReportedMisbehavior(channel channels.Channel, report network.MisbehaviorReport) {
	m.logger.Debug().
		Str("channel", channel.String()).
		Hex("misbehaving_id", logging.ID(report.OriginId())).
		Str("reason", report.Reason().String()).
		Msg("received misbehavior report")
}
