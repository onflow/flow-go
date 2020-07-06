package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type ProcessFunc func(originID flow.Identifier, entity flow.Entity)

type Requester interface {
	Request(entityID flow.Identifier, process ProcessFunc) error
}
