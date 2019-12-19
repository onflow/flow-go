// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

type proposalWrap struct {
	originID model.Identifier
	block    *flow.Block
}

type voteWrap struct {
	originID model.Identifier
	hash     crypto.Hash
}

type commitWrap struct {
	originID model.Identifier
	hash     crypto.Hash
}
