// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Auth is the outgoing handshake message
type Auth struct {
	NodeID flow.Identifier
}
