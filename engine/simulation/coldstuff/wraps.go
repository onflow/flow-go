// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type proposalWrap struct {
	originID flow.Identifier
	header   *flow.Header
}

type voteWrap struct {
	originID flow.Identifier
	blockID  flow.Identifier
}

type commitWrap struct {
	originID flow.Identifier
	blockID  flow.Identifier
}
