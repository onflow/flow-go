package provider

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

type Handler struct {
	Resource messages.Resource
	Filter   flow.IdentityFilter
	Retrieve RetrieveFunc
}

type RetrieveFunc func(flow.Identifier) (interface{}, error)
