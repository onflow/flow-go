package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Requester interface {
	EntityByID(entityID flow.Identifier) error
}
