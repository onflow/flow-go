package notifications

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/logging"
)

// LogConsumer is an implementation of the notifications consumer that logs a
// message for each event.
type LogConsumer struct {
	log zerolog.Logger
}

var _ hotstuff.Consumer = (*LogConsumer)(nil)
var _ hotstuff.TimeoutAggregationConsumer = (*LogConsumer)(nil)
var _ hotstuff.VoteAggregationConsumer = (*LogConsumer)(nil)

func NewLogConsumer(log zerolog.Logger) *LogConsumer {
	lc := &LogConsumer{
		log: log,
	}
	return lc
}

func (lc *LogConsumer) OnEventProcessed() {
	lc.log.Debug().Msg("event processed")
}

func (lc *LogConsumer) OnStart(currentView uint64) {
	lc.log.Debug().Uint64("cur_view", currentView).Msg("starting event handler")
}

func (lc *LogConsumer) OnBlockIncorporated(block *model.Block) {
	lc.logBasicBlockData(lc.log.Debug(), block).
		Msg("block incorporated")
}

func (lc *LogConsumer) OnFinalizedBlock(block *model.Block) {
	lc.logBasicBlockData(lc.log.Debug(), block).
		Msg("block finalized")
}

func (lc *LogConsumer) OnInvalidBlockDetected(err flow.Slashable[model.InvalidProposalError]) {
	invalidBlock := err.Message.InvalidProposal.Block
	lc.log.Warn().
		Str(logging.KeySuspicious, "true").
		Hex("origin_id", err.OriginID[:]).
		Uint64("block_view", invalidBlock.View).
		Hex("proposer_id", invalidBlock.ProposerID[:]).
		Hex("block_id", invalidBlock.BlockID[:]).
		Uint64("qc_block_view", invalidBlock.QC.View).
		Hex("qc_block_id", invalidBlock.QC.BlockID[:]).
		Msgf("invalid block detected: %s", err.Message.Error())
}

func (lc *LogConsumer) OnDoubleProposeDetected(block *model.Block, alt *model.Block) {
	lc.log.Warn().
		Str(logging.KeySuspicious, "true").
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Hex("alt_id", alt.BlockID[:]).
		Hex("proposer_id", block.ProposerID[:]).
		Msg("double proposal detected")
}

func (lc *LogConsumer) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	logger := lc.logBasicBlockData(lc.log.Debug(), proposal.Block).
		Uint64("cur_view", currentView)
	lastViewTC := proposal.LastViewTC
	if lastViewTC != nil {
		logger.
			Uint64("last_view_tc_view", lastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", lastViewTC.NewestQC.View).
			Hex("last_view_tc_newest_qc_block_id", logging.ID(lastViewTC.NewestQC.BlockID))
	}

	logger.Msg("processing proposal")
}

func (lc *LogConsumer) OnReceiveQc(currentView uint64, qc *flow.QuorumCertificate) {
	lc.log.Debug().
		Uint64("cur_view", currentView).
		Uint64("qc_view", qc.View).
		Hex("qc_block_id", logging.ID(qc.BlockID)).
		Msg("processing QC")
}

func (lc *LogConsumer) OnReceiveTc(currentView uint64, tc *flow.TimeoutCertificate) {
	lc.log.Debug().
		Uint64("cur_view", currentView).
		Uint64("tc_view", tc.View).
		Uint64("newest_qc_view", tc.NewestQC.View).
		Hex("newest_qc_block_id", logging.ID(tc.NewestQC.BlockID)).
		Msg("processing TC")
}

func (lc *LogConsumer) OnPartialTc(currentView uint64, partialTc *hotstuff.PartialTcCreated) {
	logger := lc.log.With().
		Uint64("cur_view", currentView).
		Uint64("view", partialTc.View).
		Uint64("qc_view", partialTc.NewestQC.View).
		Hex("qc_block_id", logging.ID(partialTc.NewestQC.BlockID))

	lastViewTC := partialTc.LastViewTC
	if lastViewTC != nil {
		logger.
			Uint64("last_view_tc_view", lastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", lastViewTC.NewestQC.View).
			Hex("last_view_tc_newest_qc_block_id", logging.ID(lastViewTC.NewestQC.BlockID))
	}

	log := logger.Logger()
	log.Debug().Msg("processing partial TC")
}

func (lc *LogConsumer) OnLocalTimeout(currentView uint64) {
	lc.log.Debug().
		Uint64("cur_view", currentView).
		Msg("processing local timeout")
}

func (lc *LogConsumer) OnViewChange(oldView, newView uint64) {
	lc.log.Debug().
		Uint64("old_view", oldView).
		Uint64("new_view", newView).
		Msg("entered new view")
}

func (lc *LogConsumer) OnQcTriggeredViewChange(oldView uint64, newView uint64, qc *flow.QuorumCertificate) {
	lc.log.Debug().
		Uint64("qc_view", qc.View).
		Hex("qc_block_id", qc.BlockID[:]).
		Uint64("old_view", oldView).
		Uint64("new_view", newView).
		Msg("QC triggered view change")
}

func (lc *LogConsumer) OnTcTriggeredViewChange(oldView uint64, newView uint64, tc *flow.TimeoutCertificate) {
	lc.log.Debug().
		Uint64("tc_view", tc.View).
		Uint64("tc_newest_qc_view", tc.NewestQC.View).
		Uint64("new_view", newView).
		Uint64("old_view", oldView).
		Msg("TC triggered view change")
}

func (lc *LogConsumer) OnStartingTimeout(info model.TimerInfo) {
	lc.log.Debug().
		Uint64("timeout_view", info.View).
		Time("timeout_cutoff", info.StartTime.Add(info.Duration)).
		Msg("timeout started")
}

func (lc *LogConsumer) OnVoteProcessed(vote *model.Vote) {
	lc.log.Debug().
		Hex("block_id", vote.BlockID[:]).
		Uint64("block_view", vote.View).
		Hex("recipient_id", vote.SignerID[:]).
		Msg("processed valid HotStuff vote")
}

func (lc *LogConsumer) OnTimeoutProcessed(timeout *model.TimeoutObject) {
	log := timeout.LogContext(lc.log).Logger()
	log.Debug().Msg("processed valid timeout object")
}

func (lc *LogConsumer) OnCurrentViewDetails(currentView, finalizedView uint64, currentLeader flow.Identifier) {
	lc.log.Info().
		Uint64("view", currentView).
		Uint64("finalized_view", finalizedView).
		Hex("current_leader", currentLeader[:]).
		Msg("current view details")
}

func (lc *LogConsumer) OnDoubleVotingDetected(vote *model.Vote, alt *model.Vote) {
	lc.log.Warn().
		Str(logging.KeySuspicious, "true").
		Uint64("vote_view", vote.View).
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("alt_id", alt.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Msg("double vote detected")
}

func (lc *LogConsumer) OnInvalidVoteDetected(err model.InvalidVoteError) {
	lc.log.Warn().
		Str(logging.KeySuspicious, "true").
		Uint64("vote_view", err.Vote.View).
		Hex("voted_block_id", err.Vote.BlockID[:]).
		Hex("voter_id", err.Vote.SignerID[:]).
		Msgf("invalid vote detected: %s", err.Error())
}

func (lc *LogConsumer) OnVoteForInvalidBlockDetected(vote *model.Vote, proposal *model.Proposal) {
	lc.log.Warn().
		Str(logging.KeySuspicious, "true").
		Uint64("vote_view", vote.View).
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Hex("proposer_id", proposal.Block.ProposerID[:]).
		Msg("vote for invalid proposal detected")
}

func (lc *LogConsumer) OnDoubleTimeoutDetected(timeout *model.TimeoutObject, alt *model.TimeoutObject) {
	lc.log.Warn().
		Str(logging.KeySuspicious, "true").
		Uint64("timeout_view", timeout.View).
		Hex("signer_id", logging.ID(timeout.SignerID)).
		Hex("timeout_id", logging.ID(timeout.ID())).
		Hex("alt_id", logging.ID(alt.ID())).
		Msg("double timeout detected")
}

func (lc *LogConsumer) OnInvalidTimeoutDetected(err model.InvalidTimeoutError) {
	log := err.Timeout.LogContext(lc.log).Logger()
	log.Warn().
		Str(logging.KeySuspicious, "true").
		Msgf("invalid timeout detected: %s", err.Error())
}

func (lc *LogConsumer) logBasicBlockData(loggerEvent *zerolog.Event, block *model.Block) *zerolog.Event {
	loggerEvent.
		Uint64("block_view", block.View).
		Hex("block_id", logging.ID(block.BlockID)).
		Hex("proposer_id", logging.ID(block.ProposerID)).
		Hex("payload_hash", logging.ID(block.PayloadHash)).
		Uint64("qc_view", block.QC.View).
		Hex("qc_block_id", logging.ID(block.QC.BlockID))

	return loggerEvent
}

func (lc *LogConsumer) OnTcConstructedFromTimeouts(tc *flow.TimeoutCertificate) {
	lc.log.Debug().
		Uint64("tc_view", tc.View).
		Uint64("newest_qc_view", tc.NewestQC.View).
		Hex("newest_qc_block_id", tc.NewestQC.BlockID[:]).
		Msg("TC constructed")
}

func (lc *LogConsumer) OnPartialTcCreated(view uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) {
	lc.log.Debug().
		Uint64("view", view).
		Uint64("newest_qc_view", newestQC.View).
		Hex("newest_qc_block_id", newestQC.BlockID[:]).
		Bool("has_last_view_tc", lastViewTC != nil).
		Msg("partial TC constructed")
}

func (lc *LogConsumer) OnNewQcDiscovered(qc *flow.QuorumCertificate) {
	lc.log.Debug().
		Uint64("qc_view", qc.View).
		Hex("qc_block_id", qc.BlockID[:]).
		Msg("new QC discovered")
}

func (lc *LogConsumer) OnNewTcDiscovered(tc *flow.TimeoutCertificate) {
	lc.log.Debug().
		Uint64("tc_view", tc.View).
		Uint64("newest_qc_view", tc.NewestQC.View).
		Hex("newest_qc_block_id", tc.NewestQC.BlockID[:]).
		Msg("new TC discovered")
}

func (lc *LogConsumer) OnOwnVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) {
	lc.log.Debug().
		Hex("block_id", blockID[:]).
		Uint64("block_view", view).
		Hex("recipient_id", recipientID[:]).
		Msg("publishing HotStuff vote")
}

func (lc *LogConsumer) OnOwnTimeout(timeout *model.TimeoutObject) {
	log := timeout.LogContext(lc.log).Logger()
	log.Debug().Msg("publishing HotStuff timeout object")
}

func (lc *LogConsumer) OnOwnProposal(header *flow.Header, targetPublicationTime time.Time) {
	lc.log.Debug().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("parent_signer_indices", header.ParentVoterIndices).
		Time("target_publication_time", targetPublicationTime).
		Msg("publishing HotStuff block proposal")
}

func (lc *LogConsumer) OnQcConstructedFromVotes(qc *flow.QuorumCertificate) {
	lc.log.Info().
		Uint64("view", qc.View).
		Hex("block_id", qc.BlockID[:]).
		Msg("QC constructed from votes")
}
