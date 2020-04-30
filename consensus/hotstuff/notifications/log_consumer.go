package notifications

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
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

func (lc *LogConsumer) OnBlockIncorporated(block *model.Block) {
	entry := lc.log.Debug().
		Uint64("block_view", block.View).
		Hex("block_id", logging.ID(block.BlockID)).
		Hex("proposer_id", logging.ID(block.ProposerID)).
		Hex("payload_hash", logging.ID(block.PayloadHash))

	if block.QC != nil {
		entry.
			Uint64("qc_view", block.QC.View).
			Hex("qc_id", logging.ID(block.QC.BlockID))
	}

	entry.Msg("block incorporated")
}

func (lc *LogConsumer) OnFinalizedBlock(block *model.Block) {
	entry := lc.log.Debug().
		Uint64("block_view", block.View).
		Hex("block_id", logging.ID(block.BlockID)).
		Hex("proposer_id", logging.ID(block.ProposerID)).
		Hex("payload_hash", logging.ID(block.PayloadHash))

	if block.QC != nil {
		entry.
			Uint64("qc_view", block.QC.View).
			Hex("qc_id", logging.ID(block.QC.BlockID))
	}

	entry.Msg("block finalized")
}

func (lc *LogConsumer) OnDoubleProposeDetected(block *model.Block, alt *model.Block) {
	lc.log.Warn().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Hex("alt_id", alt.BlockID[:]).
		Hex("proposer_id", block.ProposerID[:]).
		Msg("double proposal detected")
}

func (lc *LogConsumer) OnEnteringView(view uint64) {
	lc.log.Debug().
		Uint64("view", view).
		Msg("view entered")
}

func (lc *LogConsumer) OnSkippedAhead(view uint64) {
	lc.log.Debug().
		Uint64("view", view).
		Msg("views skipped")
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
		Uint64("fork_view", view).
		Uint64("qc_view", qc.View).
		Hex("qc_id", qc.BlockID[:]).
		Msg("fork choice generated")
}

func (lc *LogConsumer) OnBlockProposalFormed(proposal *model.Proposal) {
	block := proposal.Block
	qc := block.QC
	lc.log.Debug().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Hex("proposer_id", logging.ID(block.ProposerID)).
		Hex("payload_hash", logging.ID(block.PayloadHash)).
		Uint64("parent_view", qc.View).
		Hex("parent_id", logging.ID(qc.BlockID)).
		Msg("block proposal formed")
}

func (lc *LogConsumer) OnBlockProposalBroadcast(proposal *model.Proposal) {
	block := proposal.Block
	qc := block.QC
	lc.log.Debug().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Hex("proposer_id", logging.ID(block.ProposerID)).
		Hex("payload_hash", logging.ID(block.PayloadHash)).
		Uint64("parent_view", qc.View).
		Hex("parent_id", logging.ID(qc.BlockID)).
		Msg("block proposal broadcast")
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
