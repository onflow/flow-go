package notifications

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/logging"
)

// SlashingViolationsConsumer is an implementation of the notifications consumer that logs a
// message for any slashable offenses.
type SlashingViolationsConsumer struct {
	log zerolog.Logger
}

var _ hotstuff.ProposalViolationConsumer = (*SlashingViolationsConsumer)(nil)
var _ hotstuff.VoteAggregationViolationConsumer = (*SlashingViolationsConsumer)(nil)
var _ hotstuff.TimeoutAggregationViolationConsumer = (*SlashingViolationsConsumer)(nil)

func NewSlashingViolationsConsumer(log zerolog.Logger) *SlashingViolationsConsumer {
	return &SlashingViolationsConsumer{
		log: log,
	}
}
func (c *SlashingViolationsConsumer) OnInvalidBlockDetected(err flow.Slashable[model.InvalidProposalError]) {
	block := err.Message.InvalidProposal.Block
	c.log.Warn().
		Bool(logging.KeySuspicious, true).
		Hex("origin_id", err.OriginID[:]).
		Hex("proposer_id", block.ProposerID[:]).
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Time("block_timestamp", time.UnixMilli(int64(block.Timestamp)).UTC()).
		Msgf("OnInvalidBlockDetected: %s", err.Message.Error())
}

func (c *SlashingViolationsConsumer) OnDoubleVotingDetected(vote1 *model.Vote, vote2 *model.Vote) {
	c.log.Warn().
		Uint64("vote_view", vote1.View).
		Hex("voter_id", vote1.SignerID[:]).
		Hex("voted_block_id1", vote1.BlockID[:]).
		Hex("voted_block_id2", vote2.BlockID[:]).
		Bool(logging.KeySuspicious, true).
		Msg("OnDoubleVotingDetected")
}

func (c *SlashingViolationsConsumer) OnInvalidVoteDetected(err model.InvalidVoteError) {
	vote := err.Vote
	c.log.Warn().
		Uint64("vote_view", vote.View).
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Str("err", err.Error()).
		Bool(logging.KeySuspicious, true).
		Msg("OnInvalidVoteDetected")
}

func (c *SlashingViolationsConsumer) OnDoubleTimeoutDetected(timeout *model.TimeoutObject, altTimeout *model.TimeoutObject) {
	c.log.Warn().
		Bool(logging.KeySuspicious, true).
		Hex("timeout_signer_id", timeout.SignerID[:]).
		Uint64("timeout_view", timeout.View).
		Uint64("timeout_newest_qc_view", timeout.NewestQC.View).
		Hex("alt_signer_id", logging.ID(altTimeout.SignerID)).
		Uint64("alt_view", altTimeout.View).
		Uint64("alt_newest_qc_view", altTimeout.NewestQC.View).
		Msg("OnDoubleTimeoutDetected")
}

func (c *SlashingViolationsConsumer) OnInvalidTimeoutDetected(err model.InvalidTimeoutError) {
	timeout := err.Timeout
	c.log.Warn().
		Uint64("timeout_view", timeout.View).
		Hex("signer_id", timeout.SignerID[:]).
		Str("err", err.Error()).
		Bool(logging.KeySuspicious, true).
		Msg("OnInvalidTimeoutDetected")
}

func (c *SlashingViolationsConsumer) OnVoteForInvalidBlockDetected(vote *model.Vote, proposal *model.SignedProposal) {
	c.log.Warn().
		Uint64("vote_view", vote.View).
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Hex("proposer_id", proposal.Block.ProposerID[:]).
		Bool(logging.KeySuspicious, true).
		Msg("OnVoteForInvalidBlockDetected")
}

func (c *SlashingViolationsConsumer) OnDoubleProposeDetected(block1 *model.Block, block2 *model.Block) {
	c.log.Warn().
		Hex("proposer_id", block1.ProposerID[:]).
		Uint64("block_view", block1.View).
		Hex("block_id1", block1.BlockID[:]).
		Hex("block_id2", block2.BlockID[:]).
		Bool(logging.KeySuspicious, true).
		Msg("OnDoubleProposeDetected")
}
