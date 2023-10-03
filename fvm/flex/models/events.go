package models

import (
	"github.com/onflow/flow-go/model/flow"
)

const (
	EventFlexBlockExecuted   flow.EventType = "flex.BlockExecuted"
	EventFlexTokenDeposit    flow.EventType = "flex.FlowTokenDeposit"
	EventFlexTokenWithdrawal flow.EventType = "flex.FlowTokenWithdrawal"
)

type EventFlexBlockPayload struct {
	// ROOT hash
	// height
}

type EventFlexTokenPayload struct {
	Address *FlexAddress
	Amount  Balance
}

// TODO encode to bytes
