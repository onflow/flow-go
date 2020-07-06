package requester

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

type Request struct {
	Nonce    uint64
	TargetID flow.Identifier
	EntityID flow.Identifier
	Process  module.ProcessFunc
}
