// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Round implements a simple state cache for the consensus algorithm.
type Round interface {

	// Parent returns the parent of the block for this round.
	Parent() *flow.Header

	// Leader returns the leader of this round, who will make the block proposal
	// and to whom votes should be sent.
	Leader() *flow.Identity

	// Quorum returns the current quorum of voting stakes required for a
	// qualified majority in this round.
	Quorum() uint64

	// Participants will return a list of the other participants of the
	// consensus algorithm for this round.
	Participants() flow.IdentityList

	// Propose will store a block as the candidate for consensus in this round.
	Propose(candidate *flow.Header)

	// Candidate will return the block that we are voting for in this round.
	Candidate() *flow.Header

	// Voted will check if the given node has already voted this round.
	Voted(nodeID flow.Identifier) bool

	// Tally will count the vote stake of the given consensus node.
	Tally(nodeID flow.Identifier, stake uint64)

	// Votes will give the sum of received positive votes for this round.
	Votes() uint64
}
