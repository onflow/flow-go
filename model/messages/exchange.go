package messages

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type EntityRequest struct {
	Nonce     uint64
	EntityIDs []flow.Identifier
}

type EntityResponse struct {
	Nonce    uint64
	Entities []flow.Entity
}
