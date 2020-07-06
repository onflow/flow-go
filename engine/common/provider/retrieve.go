package provider

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type RetrieveFunc func(flow.Identifier) (flow.Entity, error)
