package alsp

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/logging"
)

// MisbehaviorReportManager is responsible for handling misbehavior reports.
// The current version is at the minimum viable product stage and only logs the reports.
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
func (m MisbehaviorReportManager) HandleReportedMisbehavior(channel channels.Channel, report network.MisbehaviorReport) {
	m.logger.Debug().
		Str("channel", channel.String()).
		Hex("misbehaving_id", logging.ID(report.OriginId())).
		Str("reason", report.Reason().String()).
		Msg("received misbehavior report")
}
