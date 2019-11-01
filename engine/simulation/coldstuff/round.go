// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/coldstuff"
)

// Round implements a simple state cache for the consensus algorithm.
type Round interface {

	// Reset will reset the current consensus state and thus start a new round.
	Reset()

	// Propose will store a candidate block as the current round subject.
	Propose(candidate *coldstuff.BlockHeader)

	// Candidate will return the header that is subject of the current round.
	Candidate() *coldstuff.BlockHeader

	// Voted will check if the given node ID has already voted this round.
	Voted(nodeID string) bool

	// Tally will count the vote of the given consensus node.
	Tally(nodeID string)

	// Votes will give a count of received positive votes for this round.
	Votes() uint
}
