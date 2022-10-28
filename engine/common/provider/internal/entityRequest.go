package internal

import "github.com/onflow/flow-go/model/flow"

type EntityRequest struct {
	OriginID  flow.Identifier
	EntityIDs []flow.Identifier
	Nonce     uint64
}
