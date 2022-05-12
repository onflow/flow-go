package notifications

import (
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/utils/logging"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// TelemetryConsumer implements the hotstuff.Consumer interface.
// consumes outbound notifications produced by the The HotStuff state machine.
// For this purpose, the TelemetryConsumer enriches the state machine's notifications:
//   * the goal is to identify all events as belonging together that were emitted during
//     a path through the state machine.
//   * A path through the state machine begins when:
//      - a vote is received
//      - a block is received
//      - a new view is started
//      - a timeout is processed
//   * Each path through the state machine is identified by a unique id.
// Generally, the TelemetryConsumer could export the collected data to a variety of backends.
// For now, we export the data to a logger.
//
// Telemetry does NOT capture slashing notifications
type TelemetryConsumer struct {
	NoopConsumer
	pathHandler *PathHandler
}

var _ hotstuff.Consumer = (*TelemetryConsumer)(nil)

func NewTelemetryConsumer(log zerolog.Logger, chain flow.ChainID) *TelemetryConsumer {
	return &TelemetryConsumer{
		pathHandler: NewPathHandler(log, chain),
	}
}

func (t *TelemetryConsumer) OnReceiveVote(currentView uint64, vote *model.Vote) {
	// TODO: update
	//       As of Consensus Voting V2, receiving a vote is not an event within the HotStuff state machine anymore.
	//t.pathHandler.StartNextPath(currentView)
	// t.pathHandler.NextStep().
	// 	Uint64("voted_block_view", vote.View).
	// 	Hex("voted_block_id", vote.BlockID[:]).
	// 	Hex("voter_id", vote.SignerID[:]).
	// 	Msg("OnReceiveVote")
}

func (t *TelemetryConsumer) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	block := proposal.Block
	t.pathHandler.StartNextPath(currentView)
	step := t.pathHandler.NextStep()
	step.
		Uint64("block_view", block.View).
		Hex("block_id", logging.ID(block.BlockID)).
		Hex("block_proposer_id", logging.ID(block.ProposerID)).
		Time("block_time", block.Timestamp)
	if block.QC != nil {
		step.
			Uint64("qc_block_view", block.QC.View).
			Hex("qc_block_id", logging.ID(block.QC.BlockID))
	}
	step.Msg("OnReceiveProposal")
}

func (t *TelemetryConsumer) OnEventProcessed() {
	if t.pathHandler.IsCurrentPathClosed() {
		return
	}
	t.pathHandler.NextStep().Msg("PathCompleted")
	t.pathHandler.NextStep().Msg("OnEventProcessed")
	t.pathHandler.CloseCurrentPath()
}

func (t *TelemetryConsumer) OnStartingTimeout(info *model.TimerInfo) {
	if info.Mode == model.ReplicaTimeout {
		// the PaceMarker starts a new ReplicaTimeout if and only if it transitions to a higher view
		t.pathHandler.StartNextPath(info.View)
	}
	t.pathHandler.NextStep().
		Str("timeout_mode", info.Mode.String()).
		Float64("timeout_duration_seconds", info.Duration.Seconds()).
		Time("timeout_cutoff", info.StartTime.Add(info.Duration)).
		Msg("OnStartingTimeout")
}

func (t *TelemetryConsumer) OnEnteringView(viewNumber uint64, leader flow.Identifier) {
	t.pathHandler.NextStep().
		Uint64("entered_view", viewNumber).
		Hex("leader", leader[:]).
		Msg("OnEnteringView")
}

func (t *TelemetryConsumer) OnReachedTimeout(info *model.TimerInfo) {
	t.pathHandler.StartNextPath(info.View)
	t.pathHandler.NextStep().
		Str("timeout_mode", info.Mode.String()).
		Time("timeout_start_time", info.StartTime).
		Float64("timeout_duration_seconds", info.Duration.Seconds()).
		Msg("OnReachedTimeout")
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

func (t *TelemetryConsumer) OnQcTriggeredViewChange(qc *flow.QuorumCertificate, newView uint64) {
	t.pathHandler.NextStep().
		Uint64("qc_block_view", qc.View).
		Uint64("next_view", newView).
		Hex("qc_block_id", qc.BlockID[:]).
		Msg("OnQcTriggeredViewChange")
}

func (t *TelemetryConsumer) OnProposingBlock(proposal *model.Proposal) {
	block := proposal.Block
	step := t.pathHandler.NextStep()
	step.
		Uint64("block_view", block.View).
		Hex("block_id", logging.ID(block.BlockID)).
		Hex("block_proposer_id", logging.ID(block.ProposerID)).
		Time("block_time", block.Timestamp)
	if block.QC != nil {
		step.
			Uint64("qc_block_view", block.QC.View).
			Hex("qc_block_id", logging.ID(block.QC.BlockID))
	}
	step.Msg("OnProposingBlock")
}

func (t *TelemetryConsumer) OnVoting(vote *model.Vote) {
	t.pathHandler.NextStep().
		Uint64("voted_block_view", vote.View).
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Msg("OnVoting")
}

func (t *TelemetryConsumer) OnForkChoiceGenerated(current_view uint64, qc *flow.QuorumCertificate) {
	t.pathHandler.NextStep().
		Uint64("block_view", current_view).
		Msg("OnForkChoiceGenerated")
	// Telemetry does not capture the details of the qc as the qc will be included in the
	// proposed block, whose details (including the qc) are captured by telemetry
}

func (t *TelemetryConsumer) OnQcConstructedFromVotes(curView uint64, qc *flow.QuorumCertificate) {
	t.pathHandler.StartNextPath(curView)
	t.pathHandler.NextStep().
		Uint64("curView", curView).
		Uint64("qc_block_view", qc.View).
		Hex("qc_block_id", qc.BlockID[:]).
		Msg("OnQcConstructedFromVotes")
}

func (t *TelemetryConsumer) OnQcIncorporated(qc *flow.QuorumCertificate) {
	t.pathHandler.NextStep().
		Uint64("qc_block_view", qc.View).
		Hex("qc_block_id", qc.BlockID[:]).
		Msg("OnQcIncorporated")
}

// PathHandler maintains a notion of the current path through the state machine.
// It allows to close a path and open new path. Each path is identified by a unique
// (randomly generated) uuid. Along each path, we can capture information about relevant
// Steps (each step is represented by a Zerolog Event).
// In case there is no currently open path, the PathHandler still returns a Step,
// but such steps are logged as telemetry errors.
type PathHandler struct {
	chain flow.ChainID
	log   zerolog.Logger

	// currentPath holds a Zerolog Context with the information about the current path.
	// We represent the case where the current path has been closed by nil value.
	currentPath *zerolog.Context
}

// NewPathHandler instantiate a new PathHandler.
// The PathHandler has no currently open path
func NewPathHandler(log zerolog.Logger, chain flow.ChainID) *PathHandler {
	return &PathHandler{
		chain:       chain,
		log:         log.With().Str("hotstuff", "telemetry").Str("chain", chain.String()).Logger(),
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

// IsCurrentPathOpen if and only is the most recently started path has been closed.
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
