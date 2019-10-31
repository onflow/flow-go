// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/coldstuff"
)

type proposalWrap struct {
	originID string
	proposal *coldstuff.BlockProposal
}

type voteWrap struct {
	originID string
	vote     *coldstuff.BlockVote
}

type commitWrap struct {
	originID string
	commit   *coldstuff.BlockCommit
}
