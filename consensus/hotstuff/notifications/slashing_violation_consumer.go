package notifications

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/utils/logging"
)

// SlashingViolationsConsumer is an implementation of the notifications consumer that logs a
// message for any slashable offenses.
type SlashingViolationsConsumer struct {
	NoopConsumer
	log zerolog.Logger
}

func NewSlashingViolationsConsumer(log zerolog.Logger) *SlashingViolationsConsumer {
	return &SlashingViolationsConsumer{
		log: log,
	}
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

func (c *SlashingViolationsConsumer) OnInvalidTimeoutDetected(err model.InvalidTimeoutError) {
	timeout := err.Timeout
	c.log.Warn().
		Uint64("timeout_view", timeout.View).
		Hex("signer_id", timeout.SignerID[:]).
		Str("err", err.Error()).
		Bool(logging.KeySuspicious, true).
		Msg("OnInvalidTimeoutDetected")
}

func (c *SlashingViolationsConsumer) OnVoteForInvalidBlockDetected(vote *model.Vote, proposal *model.Proposal) {
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
