package notifications

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// LogConsumer is an implementation of the notifications consumer that logs a
// message for each event.
type LogConsumer struct {
	log zerolog.Logger
}

func NewLogConsumer(log zerolog.Logger) *LogConsumer {
	lc := &LogConsumer{
		log: log,
	}
	return lc
}

func (lc *LogConsumer) OnEventProcessed() {
	lc.log.Debug().Msg("event processed")
}

func (lc *LogConsumer) OnBlockIncorporated(block *model.Block) {
	lc.logBasicBlockInfo(lc.log.Debug(), block).
		Msg("block incorporated")
}

func (lc *LogConsumer) OnFinalizedBlock(block *model.Block) {
	lc.logBasicBlockInfo(lc.log.Debug(), block).
		Msg("block finalized")
}

func (lc *LogConsumer) OnDoubleProposeDetected(block *model.Block, alt *model.Block) {
	lc.log.Warn().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Hex("alt_id", alt.BlockID[:]).
		Hex("proposer_id", block.ProposerID[:]).
		Msg("double proposal detected")
}

func (lc *LogConsumer) OnReceiveVote(currentView uint64, vote *model.Vote) {
	lc.log.Debug().
		Uint64("cur_view", currentView).
		Uint64("vote_view", vote.View).
		Hex("vote_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Msg("processing vote")
}

func (lc *LogConsumer) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	lc.logBasicBlockInfo(lc.log.Debug(), proposal.Block).
		Uint64("cur_view", currentView).
		Msg("processing proposal")
}

func (lc *LogConsumer) OnEnteringView(view uint64, leader flow.Identifier) {
	lc.log.Debug().
		Uint64("view", view).
		Hex("leader", leader[:]).
		Msg("view entered")
}

func (lc *LogConsumer) OnQcTriggeredViewChange(qc *model.QuorumCertificate, newView uint64) {
	lc.log.Debug().
		Uint64("qc_view", qc.View).
		Hex("qc_id", qc.BlockID[:]).
		Uint64("new_view", newView).
		Msg("QC triggered view change")
}

func (lc *LogConsumer) OnProposingBlock(block *model.Proposal) {
	lc.logBasicBlockInfo(lc.log.Debug(), block.Block).
		Msg("proposing block")
}

func (lc *LogConsumer) OnVoting(vote *model.Vote) {
	lc.log.Debug().
		Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Msg("voting for block")
}

func (lc *LogConsumer) OnQcConstructedFromVotes(qc *model.QuorumCertificate) {
	lc.log.Debug().
		Uint64("qc_view", qc.View).
		Hex("qc_id", qc.BlockID[:]).
		Msg("QC constructed from votes")
}

func (lc *LogConsumer) OnStartingTimeout(info *model.TimerInfo) {
	lc.log.Debug().
		Uint64("timeout_view", info.View).
		Time("timeout_cutoff", info.StartTime.Add(info.Duration)).
		Str("timeout_mode", info.Mode.String()).
		Msg("timeout started")
}

func (lc *LogConsumer) OnReachedTimeout(info *model.TimerInfo) {
	lc.log.Debug().
		Uint64("timeout_view", info.View).
		Time("timeout_cutoff", info.StartTime.Add(info.Duration)).
		Str("timeout_mode", info.Mode.String()).
		Msg("timeout reached")
}

func (lc *LogConsumer) OnQcIncorporated(qc *model.QuorumCertificate) {
	lc.log.Debug().
		Uint64("qc_view", qc.View).
		Hex("qc_id", qc.BlockID[:]).
		Msg("QC incorporated")
}

func (lc *LogConsumer) OnForkChoiceGenerated(view uint64, qc *model.QuorumCertificate) {
	lc.log.Debug().
		Uint64("proposal_view", view).
		Uint64("qc_view", qc.View).
		Hex("qc_id", qc.BlockID[:]).
		Msg("fork choice generated")
}

func (lc *LogConsumer) OnDoubleVotingDetected(vote *model.Vote, alt *model.Vote) {
	lc.log.Warn().
		Uint64("vote_view", vote.View).
		Hex("vote_id", vote.BlockID[:]).
		Hex("alt_id", alt.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Msg("double vote detected")
}

func (lc *LogConsumer) OnInvalidVoteDetected(vote *model.Vote) {
	lc.log.Warn().
		Uint64("vote_view", vote.View).
		Hex("vote_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Msg("invalid vote detected")
}

func (lc *LogConsumer) logBasicBlockInfo(loggerEvent *zerolog.Event, block *model.Block) *zerolog.Event {
	loggerEvent.
		Uint64("block_view", block.View).
		Hex("block_id", logging.ID(block.BlockID)).
		Hex("proposer_id", logging.ID(block.ProposerID)).
		Hex("payload_hash", logging.ID(block.PayloadHash))
	if block.QC != nil {
		loggerEvent.
			Uint64("qc_view", block.QC.View).
			Hex("qc_id", logging.ID(block.QC.BlockID))
	}
	return loggerEvent
}
