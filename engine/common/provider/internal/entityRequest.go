package internal

import "github.com/onflow/flow-go/model/flow"

type EntityRequest struct {
	OriginId  flow.Identifier
	EntityIds []flow.Identifier
	Nonce     uint64
}
