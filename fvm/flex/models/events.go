package models

import (
	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/model/flow"
)

const (
	EventFlexTokenDeposit    flow.EventType = "flex.FlowTokenDeposit"
	EventFlexTokenWithdrawal flow.EventType = "flex.FlowTokenWithdrawal"
)

func NewEventFlexTokenDeposit() cadence.Event {
	// TODO fill me in
	// []Value
	// eventType := cadence.NewEventType()
	// cadence.NewEvent().WithType()
	return cadence.NewEvent(nil).WithType(nil)
}
