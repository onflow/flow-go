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
var _ hotstuff.TimeoutCollectorConsumer = (*LogConsumer)(nil)

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
	lc.logBasicBlockData(lc.log.Debug(), block).
		Msg("block incorporated")
}

func (lc *LogConsumer) OnFinalizedBlock(block *model.Block) {
	lc.logBasicBlockData(lc.log.Debug(), block).
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
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Msg("processing vote")
}

func (lc *LogConsumer) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	lc.logBasicBlockData(lc.log.Debug(), proposal.Block).
		Uint64("cur_view", currentView).
		Msg("processing proposal")
}

func (lc *LogConsumer) OnEnteringView(view uint64, leader flow.Identifier) {
	lc.log.Debug().
		Uint64("view", view).
		Hex("leader", leader[:]).
		Msg("view entered")
}

func (lc *LogConsumer) OnQcTriggeredViewChange(qc *flow.QuorumCertificate, newView uint64) {
	lc.log.Debug().
		Uint64("qc_view", qc.View).
		Hex("qc_id", qc.BlockID[:]).
		Uint64("new_view", newView).
		Msg("QC triggered view change")
}

func (lc *LogConsumer) OnTcTriggeredViewChange(tc *flow.TimeoutCertificate, newView uint64) {
	lc.log.Debug().
		Uint64("tc_view", tc.View).
		Uint64("tc_newest_qc_view", tc.NewestQC.View).
		Uint64("new_view", newView).
		Msg("TC triggered view change")
}

func (lc *LogConsumer) OnQcConstructedFromVotes(curView uint64, qc *flow.QuorumCertificate) {
	lc.log.Debug().
		Uint64("cur_view", curView).
		Uint64("qc_view", qc.View).
		Hex("qc_id", qc.BlockID[:]).
		Msg("QC constructed from votes")
}

func (lc *LogConsumer) OnStartingTimeout(info model.TimerInfo) {
	lc.log.Debug().
		Uint64("timeout_view", info.View).
		Time("timeout_cutoff", info.StartTime.Add(info.Duration)).
		Msg("timeout started")
}

func (lc *LogConsumer) OnReachedTimeout(info model.TimerInfo) {
	lc.log.Debug().
		Uint64("timeout_view", info.View).
		Uint64("tick", info.Tick).
		Time("timeout_cutoff", info.StartTime.Add(info.Duration)).
		Msg("timeout reached")
}

func (lc *LogConsumer) OnQcIncorporated(qc *flow.QuorumCertificate) {
	lc.log.Debug().
		Uint64("qc_view", qc.View).
		Hex("qc_id", qc.BlockID[:]).
		Msg("QC incorporated")
}

func (lc *LogConsumer) OnDoubleVotingDetected(vote *model.Vote, alt *model.Vote) {
	lc.log.Warn().
		Uint64("vote_view", vote.View).
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("alt_id", alt.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Msg("double vote detected")
}

func (lc *LogConsumer) OnInvalidVoteDetected(vote *model.Vote) {
	lc.log.Warn().
		Uint64("vote_view", vote.View).
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Msg("invalid vote detected")
}

func (lc *LogConsumer) OnVoteForInvalidBlockDetected(vote *model.Vote, proposal *model.Proposal) {
	lc.log.Warn().
		Uint64("vote_view", vote.View).
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Hex("proposer_id", proposal.Block.ProposerID[:]).
		Msg("vote for invalid proposal detected")
}

func (lc *LogConsumer) OnDoubleTimeoutDetected(timeout *model.TimeoutObject, alt *model.TimeoutObject) {
	lc.log.Warn().
		Uint64("timeout_view", timeout.View).
		Hex("signer_id", logging.ID(timeout.SignerID)).
		Hex("timeout_id", logging.ID(timeout.ID())).
		Hex("alt_id", logging.ID(alt.ID())).
		Msg("double timeout detected")
}

func (lc *LogConsumer) OnInvalidTimeoutDetected(timeout *model.TimeoutObject) {
	lc.log.Warn().
		Uint64("timeout_view", timeout.View).
		Hex("signer_id", timeout.SignerID[:]).
		Msg("invalid timeout detected")
}

func (lc *LogConsumer) logBasicBlockData(loggerEvent *zerolog.Event, block *model.Block) *zerolog.Event {
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
	lc.log.Info().
		Hex("block_id", blockID[:]).
		Uint64("block_view", view).
		Hex("recipient_id", recipientID[:]).
		Msg("publishing HotStuff vote")
}

func (lc *LogConsumer) OnOwnTimeout(timeout *model.TimeoutObject, timeoutTick uint64) {
	log := timeout.LogContext(lc.log).Logger()
	log.Info().Msg("publishing HotStuff timeout object")
}

func (lc *LogConsumer) OnOwnProposal(header *flow.Header, targetPublicationTime time.Time) {
	lc.log.Info().
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
