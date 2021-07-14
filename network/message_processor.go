// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package network

import (
	"github.com/onflow/flow-go/model/flow"
)

type MessageProcessor interface {
	Process(channel Channel, originID flow.Identifier, message interface{}) error
}
