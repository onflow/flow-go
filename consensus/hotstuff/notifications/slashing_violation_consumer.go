package notifications

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// LogConsumer is an implementation of the notifications consumer that logs a
// message for each event.
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
		Msg("OnDoubleVotingDetected")
}

func (c *SlashingViolationsConsumer) OnInvalidVoteDetected(vote *model.Vote) {
	c.log.Warn().
		Uint64("vote_view", vote.View).
		Hex("voted_block_id", vote.BlockID[:]).
		Hex("voter_id", vote.SignerID[:]).
		Msg("OnInvalidVoteDete ted")
}

func (c *SlashingViolationsConsumer) OnDoubleProposeDetected(block1 *model.Block, block2 *model.Block) {
	c.log.Warn().
		Hex("proposer_id", block1.ProposerID[:]).
		Uint64("block_view", block1.View).
		Hex("block_id1", block1.BlockID[:]).
		Hex("block_id2", block2.BlockID[:]).
		Msg("OnDoubleProposeDetected")
}
