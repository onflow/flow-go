// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

import (
	"github.com/dapperlabs/flow-go/model"
)

// Response represents an opaque system layer event.
type Response struct {
	EngineID  uint8
	EventID   []byte
	OriginID  model.Identifier
	TargetIDs []model.Identifier
	Payload   []byte
}
