package notifications

import (
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/logging"
)

// TelemetryConsumer implements the hotstuff.Consumer interface.
// consumes outbound notifications produced by the HotStuff state machine.
// For this purpose, the TelemetryConsumer enriches the state machine's notifications:
//   - The goal is to identify all events as belonging together that were emitted during
//     a path through the state machine.
//   - A path through the state machine begins when:
//     -- a block has been received
//     -- a QC has been constructed
//     -- a TC has been constructed
//     -- a partial TC has been constructed
//     -- a local timeout has been initiated
//   - Each path through the state machine is identified by a unique id.
//
// Additionally, the TelemetryConsumer reports events related to vote and timeout aggregation
// but those events are not bound to a path, so they are reported differently.
// Generally, the TelemetryConsumer could export the collected data to a variety of backends.
// For now, we export the data to a logger.
//
// Telemetry does NOT capture slashing notifications
type TelemetryConsumer struct {
	NoopTimeoutCollectorConsumer
	NoopVoteCollectorConsumer
	pathHandler  *PathHandler
	noPathLogger zerolog.Logger
}

// Telemetry implements consumers for _all happy-path_ interfaces in consensus/hotstuff/notifications/telemetry.go:
var _ hotstuff.ParticipantConsumer = (*TelemetryConsumer)(nil)
var _ hotstuff.CommunicatorConsumer = (*TelemetryConsumer)(nil)
var _ hotstuff.FinalizationConsumer = (*TelemetryConsumer)(nil)
var _ hotstuff.VoteCollectorConsumer = (*TelemetryConsumer)(nil)
var _ hotstuff.TimeoutCollectorConsumer = (*TelemetryConsumer)(nil)

// NewTelemetryConsumer creates consumer that reports telemetry events using logger backend.
// Logger MUST include `chain` parameter as part of log context with corresponding chain ID to correctly map telemetry events to chain.
func NewTelemetryConsumer(log zerolog.Logger) *TelemetryConsumer {
	pathHandler := NewPathHandler(log)
	return &TelemetryConsumer{
		pathHandler:  pathHandler,
		noPathLogger: pathHandler.log,
	}
}

func (t *TelemetryConsumer) OnStart(currentView uint64) {
	t.pathHandler.StartNextPath(currentView)
	t.pathHandler.NextStep().Msg("OnStart")
}

func (t *TelemetryConsumer) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	block := proposal.Block
	t.pathHandler.StartNextPath(currentView)
	step := t.pathHandler.NextStep().
		Uint64("block_view", block.View).
		Hex("block_id", logging.ID(block.BlockID)).
		Hex("block_proposer_id", logging.ID(block.ProposerID)).
		Time("block_time", block.Timestamp).
		Uint64("qc_view", block.QC.View).
		Hex("qc_block_id", logging.ID(block.QC.BlockID))

	lastViewTC := proposal.LastViewTC
	if lastViewTC != nil {
		step.
			Uint64("last_view_tc_view", lastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", lastViewTC.NewestQC.View).
			Hex("last_view_tc_newest_qc_block_id", logging.ID(lastViewTC.NewestQC.BlockID))
	}

	step.Msg("OnReceiveProposal")
}

func (t *TelemetryConsumer) OnReceiveQc(currentView uint64, qc *flow.QuorumCertificate) {
	t.pathHandler.StartNextPath(currentView)
	t.pathHandler.NextStep().
		Uint64("qc_view", qc.View).
		Hex("qc_block_id", logging.ID(qc.BlockID)).
		Msg("OnReceiveQc")
}

func (t *TelemetryConsumer) OnReceiveTc(currentView uint64, tc *flow.TimeoutCertificate) {
	t.pathHandler.StartNextPath(currentView)
	t.pathHandler.NextStep().
		Uint64("view", tc.View).
		Uint64("newest_qc_view", tc.NewestQC.View).
		Hex("newest_qc_block_id", logging.ID(tc.NewestQC.BlockID)).
		Msg("OnReceiveTc")
}

func (t *TelemetryConsumer) OnPartialTc(currentView uint64, partialTc *hotstuff.PartialTcCreated) {
	t.pathHandler.StartNextPath(currentView)
	step := t.pathHandler.NextStep().
		Uint64("view", partialTc.View).
		Uint64("newest_qc_view", partialTc.NewestQC.View).
		Hex("newest_qc_block_id", logging.ID(partialTc.NewestQC.BlockID))

	lastViewTC := partialTc.LastViewTC
	if lastViewTC != nil {
		step.
			Uint64("last_view_tc_view", lastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", lastViewTC.NewestQC.View).
			Hex("last_view_tc_newest_qc_block_id", logging.ID(lastViewTC.NewestQC.BlockID))
	}

	step.Msg("OnPartialTc")
}

func (t *TelemetryConsumer) OnLocalTimeout(currentView uint64) {
	t.pathHandler.StartNextPath(currentView)
	t.pathHandler.NextStep().Msg("OnLocalTimeout")
}

func (t *TelemetryConsumer) OnEventProcessed() {
	if t.pathHandler.IsCurrentPathClosed() {
		return
	}
	t.pathHandler.NextStep().Msg("PathCompleted")
	t.pathHandler.NextStep().Msg("OnEventProcessed")
	t.pathHandler.CloseCurrentPath()
}

func (t *TelemetryConsumer) OnStartingTimeout(info model.TimerInfo) {
	t.pathHandler.NextStep().
		Float64("timeout_duration_seconds", info.Duration.Seconds()).
		Time("timeout_cutoff", info.StartTime.Add(info.Duration)).
		Msg("OnStartingTimeout")
}

func (t *TelemetryConsumer) OnBlockIncorporated(block *model.Block) {
	t.pathHandler.NextStep().
		Hex("block_id", logging.ID(block.BlockID)).
		Msg("OnBlockIncorporated")
}

func (t *TelemetryConsumer) OnFinalizedBlock(block *model.Block) {
	t.pathHandler.NextStep().
		Hex("block_id", logging.ID(block.BlockID)).
		Msg("OnFinalizedBlock")
}

func (t *TelemetryConsumer) OnQcTriggeredViewChange(oldView uint64, newView uint64, qc *flow.QuorumCertificate) {
	t.pathHandler.NextStep().
		Uint64("qc_view", qc.View).
		Uint64("old_view", oldView).
		Uint64("next_view", newView).
		Hex("qc_block_id", qc.BlockID[:]).
		Msg("OnQcTriggeredViewChange")
}

func (t *TelemetryConsumer) OnTcTriggeredViewChange(oldView uint64, newView uint64, tc *flow.TimeoutCertificate) {
	t.pathHandler.NextStep().
		Uint64("tc_view", tc.View).
		Uint64("old_view", oldView).
		Uint64("next_view", newView).
		Uint64("tc_newest_qc_view", tc.NewestQC.View).
		Hex("tc_newest_qc_block_id", tc.NewestQC.BlockID[:]).
		Msg("OnTcTriggeredViewChange")
}

func (t *TelemetryConsumer) OnOwnVote(blockID flow.Identifier, view uint64, _ []byte, recipientID flow.Identifier) {
	t.pathHandler.NextStep().
		Uint64("voted_block_view", view).
		Hex("voted_block_id", logging.ID(blockID)).
		Hex("recipient_id", logging.ID(recipientID)).
		Msg("OnOwnVote")
}

func (t *TelemetryConsumer) OnOwnProposal(proposal *flow.Header, targetPublicationTime time.Time) {
	step := t.pathHandler.NextStep().
		Uint64("block_view", proposal.View).
		Hex("block_id", logging.ID(proposal.ID())).
		Hex("block_proposer_id", logging.ID(proposal.ProposerID)).
		Time("block_time", proposal.Timestamp).
		Uint64("qc_view", proposal.ParentView).
		Hex("qc_block_id", logging.ID(proposal.ParentID)).
		Time("targetPublicationTime", targetPublicationTime)
	lastViewTC := proposal.LastViewTC
	if lastViewTC != nil {
		step.
			Uint64("last_view_tc_view", lastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", lastViewTC.NewestQC.View).
			Hex("last_view_tc_newest_qc_block_id", logging.ID(lastViewTC.NewestQC.BlockID))
	}
	step.Msg("OnOwnProposal")
}

func (t *TelemetryConsumer) OnOwnTimeout(timeout *model.TimeoutObject) {
	step := t.pathHandler.NextStep().
		Uint64("view", timeout.View).
		Uint64("timeout_tick", timeout.TimeoutTick).
		Uint64("newest_qc_view", timeout.NewestQC.View).
		Hex("newest_qc_block_id", logging.ID(timeout.NewestQC.BlockID))

	lastViewTC := timeout.LastViewTC
	if lastViewTC != nil {
		step.
			Uint64("last_view_tc_view", lastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", lastViewTC.NewestQC.View).
			Hex("last_view_tc_newest_qc_block_id", logging.ID(lastViewTC.NewestQC.BlockID))
	}
	step.Msg("OnOwnTimeout")
}

func (t *TelemetryConsumer) OnVoteProcessed(vote *model.Vote) {
	t.noPathLogger.Info().
		Uint64("voted_block_view", vote.View).
		Hex("voted_block_id", logging.ID(vote.BlockID)).
		Hex("signer_id", logging.ID(vote.SignerID)).
		Msg("OnVoteProcessed")
}

func (t *TelemetryConsumer) OnTimeoutProcessed(timeout *model.TimeoutObject) {
	step := t.noPathLogger.Info().
		Uint64("view", timeout.View).
		Uint64("timeout_tick", timeout.TimeoutTick).
		Uint64("newest_qc_view", timeout.NewestQC.View).
		Hex("newest_qc_block_id", logging.ID(timeout.NewestQC.BlockID)).
		Hex("signer_id", logging.ID(timeout.SignerID))
	lastViewTC := timeout.LastViewTC
	if lastViewTC != nil {
		step.
			Uint64("last_view_tc_view", lastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", lastViewTC.NewestQC.View).
			Hex("last_view_tc_newest_qc_block_id", logging.ID(lastViewTC.NewestQC.BlockID))
	}
	step.Msg("OnTimeoutProcessed")
}

func (t *TelemetryConsumer) OnCurrentViewDetails(currentView, finalizedView uint64, currentLeader flow.Identifier) {
	t.pathHandler.NextStep().
		Uint64("view", currentView).
		Uint64("finalized_view", finalizedView).
		Hex("current_leader", currentLeader[:]).
		Msg("OnCurrentViewDetails")
}

func (t *TelemetryConsumer) OnViewChange(oldView, newView uint64) {
	t.pathHandler.NextStep().
		Uint64("old_view", oldView).
		Uint64("new_view", newView).
		Msg("OnViewChange")
}

// PathHandler maintains a notion of the current path through the state machine.
// It allows to close a path and open new path. Each path is identified by a unique
// (randomly generated) uuid. Along each path, we can capture information about relevant
// Steps (each step is represented by a Zerolog Event).
// In case there is no currently open path, the PathHandler still returns a Step,
// but such steps are logged as telemetry errors.
type PathHandler struct {
	log zerolog.Logger

	// currentPath holds a Zerolog Context with the information about the current path.
	// We represent the case where the current path has been closed by nil value.
	currentPath *zerolog.Context
}

// NewPathHandler instantiate a new PathHandler.
// The PathHandler has no currently open path
// Logger MUST include `chain` parameter as part of log context with corresponding chain ID to correctly map telemetry events to chain.
func NewPathHandler(log zerolog.Logger) *PathHandler {
	return &PathHandler{
		log:         log.With().Str("component", "hotstuff.telemetry").Logger(),
		currentPath: nil,
	}
}

// StartNextPath starts a new Path. Implicitly closes previous path if still open.
// Returns self-reference for chaining
func (p *PathHandler) StartNextPath(view uint64) *PathHandler {
	if p.currentPath != nil {
		p.NextStep().Msg("PathCompleted")
	}
	c := p.log.With().
		Str("path_id", uuid.New().String()).
		Uint64("view", view)
	p.currentPath = &c
	return p
}

// CloseCurrentPath closes the current path. Repeated calls to CloseCurrentPath are handled.
// All Details hereafter, until a new Path is started, are logged as telemetry Errors.
// Returns self-reference for chaining
func (p *PathHandler) CloseCurrentPath() *PathHandler {
	p.currentPath = nil
	return p
}

// IsCurrentPathClosed if and only if the most recently started path has been closed.
func (p *PathHandler) IsCurrentPathClosed() bool {
	return p.currentPath == nil
}

// NextStep returns a Zerolog event for the currently open path. If the current path
// is closed, the event will be logged as telemetry error.
func (p *PathHandler) NextStep() *zerolog.Event {
	if p.currentPath == nil {
		l := p.log.With().Str("error", "no path").Logger()
		return l.Error()
	}
	l := p.currentPath.Logger()
	return l.Info()
}
