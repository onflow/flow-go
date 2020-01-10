// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type proposalWrap struct {
	originID flow.Identifier
	block    *flow.Block
}

type voteWrap struct {
	originID flow.Identifier
	blockID  flow.Identifier
}

type commitWrap struct {
	originID flow.Identifier
	blockID  flow.Identifier
}
