package genericstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Proposal struct {
	originID flow.Identifier
	flow.Header
}

type Vote struct {
	originID flow.Identifier
	blockID  flow.Identifier
}

type Commit struct {
	originID flow.Identifier
	blockID  flow.Identifier
}
