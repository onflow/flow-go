package some

import (
	"time"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/queue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

type Engine struct {
	unit           *engine.Unit
	core           Core
	incomingEvents chan *queue.Event
}

func (e *Engine) Process(originID flow.Identifier, evt interface{}) error {
	queue.ReceiveEvent(e.incomingEvents, originID, evt)
	return nil
}

func (e *Engine) Ready() <-chan struct{} {
	handlers := []*queue.Handler{
		// each handler tries to match a certain message, and
		// handles it if matched. Just like pattern matching
		&queue.Handler{
			Match: func(evt interface{}) bool {
				switch evt.(type) {
				case *flow.ExecutionReceipt:
					return true
				}
				return false
			},
			Process: func(originID flow.Identifier, evt interface{}) {
				e.core.ProcessReceipt(originID, evt.(*flow.ExecutionReceipt))
			},
		},
		&queue.Handler{
			Match: func(evt interface{}) bool {
				switch evt.(type) {
				case *flow.ResultApproval:
					return true
				}
				return false
			},
			Process: func(originID flow.Identifier, evt interface{}) {
				e.core.ProcessApproval(originID, evt.(*flow.ResultApproval))
			},
		},
		&queue.Handler{
			Match: func(evt interface{}) bool {
				switch evt.(type) {
				case *messages.ApprovalResponse:
					return true
				}
				return false
			},
			Process: func(originID flow.Identifier, evt interface{}) {
				e.core.ProcessApprovalResponse(originID, evt.(*messages.ApprovalResponse))
			},
		},
	}

	checkSealingTicker := make(chan struct{})
	defer close(checkSealingTicker)
	e.unit.LaunchPeriodically(func() {
		checkSealingTicker <- struct{}{}
	}, 2*time.Second, 120*time.Second)

	queue.ConsumeEvents(e.incomingEvents, handlers, e.unit.Quit(), checkSealingTicker, e.core.CheckSealing)
	return e.unit.Ready()
}

type Core struct {
}

func (c *Core) ProcessReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) {}
func (c *Core) ProcessApproval(originID flow.Identifier, approval *flow.ResultApproval) {}
func (c *Core) ProcessApprovalResponse(originID flow.Identifier, approvalResponse *messages.ApprovalResponse) {
}
func (c *Core) CheckSealing() {}
