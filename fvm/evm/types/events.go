package types

import (
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/onflow/flow-go/model/flow"
)

const (
	EventTypeBlockExecuted       flow.EventType = "evm.BlockExecuted"
	EventTypeTransactionExecuted flow.EventType = "evm.TransactionExecuted"
	EventTypeFlowTokenDeposit    flow.EventType = "evm.FlowTokenDeposit"
	EventTypeFlowTokenWithdrawal flow.EventType = "evm.FlowTokenWithdrawal"
)

type EventPayload interface {
	Encode() ([]byte, error)
}

type Event struct {
	Etype   flow.EventType
	Payload EventPayload
}

type FlowTokenEventPayload struct {
	Address Address
	Amount  Balance
}

func (p *FlowTokenEventPayload) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(p)
}

func NewFlowTokenDepositEvent(address Address, amount Balance) *Event {
	return &Event{
		Etype: EventTypeFlowTokenDeposit,
		Payload: &FlowTokenEventPayload{
			Address: address,
			Amount:  amount,
		},
	}
}

func NewFlowTokenWithdrawalEvent(address Address, amount Balance) *Event {
	return &Event{
		Etype: EventTypeFlowTokenWithdrawal,
		Payload: &FlowTokenEventPayload{
			Address: address,
			Amount:  amount,
		},
	}
}

type TransactionExecutedPayload struct {
	BlockHeight uint64
	Result      *Result
}

func (p *TransactionExecutedPayload) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(p)
}

func NewTransactionExecutedEvent(
	height uint64,
	result *Result,
) *Event {
	return &Event{
		Etype: EventTypeTransactionExecuted,
		Payload: &TransactionExecutedPayload{
			BlockHeight: height,
			Result:      result,
		},
	}
}

type BlockExecutedEventPayload struct {
	Block *Block
}

func (p *BlockExecutedEventPayload) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(p)
}

func NewBlockExecutedEvent(block *Block) *Event {
	return &Event{
		Etype: EventTypeBlockExecuted,
		Payload: &BlockExecutedEventPayload{
			Block: block,
		},
	}
}
