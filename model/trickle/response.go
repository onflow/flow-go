// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Response represents an opaque system layer event.
type Response struct {
	ChannelID uint8
	EventID   []byte
	OriginID  flow.Identifier
	TargetIDs []flow.Identifier
	Payload   []byte
}
