package hotstuff

import (
	"github.com/onflow/flow-go/module"
)

// EventLoop performs buffer and processing of incoming proposals and QCs.
type EventLoop interface {
	module.HotStuff
	TimeoutCollectorConsumer
	VoteCollectorConsumer
}
