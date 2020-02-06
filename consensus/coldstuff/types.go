package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Vote struct {
	OriginID flow.Identifier
	BlockID  flow.Identifier
}

type Commit struct {
	OriginID flow.Identifier
	BlockID  flow.Identifier
}
