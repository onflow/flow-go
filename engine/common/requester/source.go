package requester

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

type Source struct {
	Resource messages.Resource
	Selector flow.IdentityFilter
}
