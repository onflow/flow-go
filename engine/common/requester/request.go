package requester

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Request struct {
	Nonce    uint64
	TargetID flow.Identifier
	EntityID flow.Identifier
	Process  ProcessFunc
}

// TODO: should we use `flow.Entity` here? It would make sense, but it
// would mean we can't use the normal `engine.Process` function.
type ProcessFunc func(originID flow.Identifier, entity interface{})
