package models

import (
	"github.com/onflow/flow-go/model/flow"
)

const (
	EventFlexTokenDeposit    flow.EventType = "flex.FlowTokenDeposit"
	EventFlexTokenWithdrawal flow.EventType = "flex.FlowTokenWithdrawal"
)

type EventFlexTokenPayload struct {
	Address *FlexAddress
	Amount  Balance
}

// TODO encode to bytes
