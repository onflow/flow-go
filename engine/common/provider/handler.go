package engine

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type HandlerFunc func(flow.Identifier) (interface{}, error)
