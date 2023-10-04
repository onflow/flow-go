package models

import (
	"bytes"
	"io"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/flow-go/model/flow"
)

const (
	EventTypeFlexBlockExecuted   flow.EventType = "flex.BlockExecuted"
	EventTypeFlexEVMLog          flow.EventType = "flex.EVMLog"
	EventTypeFlowTokenDeposit    flow.EventType = "flex.FlowTokenDeposit"
	EventTypeFlowTokenWithdrawal flow.EventType = "flex.FlowTokenWithdrawal"
)

type EventPayload interface {
	RLPEncode() ([]byte, error)
}

type Event struct {
	Etype   flow.EventType
	Payload EventPayload
}

type FlowTokenEventPayload struct {
	address FlexAddress
	amount  Balance
}

func (p *FlowTokenEventPayload) RLPEncode() ([]byte, error) {
	var data bytes.Buffer
	dataWriter := io.Writer(&data)
	_, err := dataWriter.Write(p.address[:])
	if err != nil {
		return nil, err
	}

	_, err = dataWriter.Write(p.amount.Encode())
	if err != nil {
		return nil, err
	}

	var encoded bytes.Buffer
	encWriter := io.Writer(&encoded)
	err = rlp.Encode(encWriter, data)
	return encoded.Bytes(), err
}

func NewFlowTokenDepositEvent(address FlexAddress, amount Balance) *Event {
	return &Event{
		Etype: EventTypeFlowTokenDeposit,
		Payload: &FlowTokenEventPayload{
			address: address,
			amount:  amount,
		},
	}
}

func NewFlowTokenWithdrawalEvent(address FlexAddress, amount Balance) *Event {
	return &Event{
		Etype: EventTypeFlowTokenWithdrawal,
		Payload: &FlowTokenEventPayload{
			address: address,
			amount:  amount,
		},
	}
}

type EVMLogEventPayload struct {
	log *types.Log
}

func (p *EVMLogEventPayload) RLPEncode() ([]byte, error) {
	var encoded bytes.Buffer
	encWriter := io.Writer(&encoded)
	err := p.log.EncodeRLP(encWriter)
	return encoded.Bytes(), err
}

func NewEVMLogEvent(log *types.Log) *Event {
	return &Event{
		Etype: EventTypeFlexEVMLog,
		Payload: &EVMLogEventPayload{
			log: log,
		},
	}
}

type BlockExecutedEventPayload struct {
	block *FlexBlock
}

func (p *BlockExecutedEventPayload) RLPEncode() ([]byte, error) {
	var encoded bytes.Buffer
	encWriter := io.Writer(&encoded)
	err := rlp.Encode(encWriter, p.block.ToBytes())
	return encoded.Bytes(), err
}

func NewBlockExecutedEvent(block *FlexBlock) *Event {
	return &Event{
		Etype: EventTypeFlexBlockExecuted,
		Payload: &BlockExecutedEventPayload{
			block: block,
		},
	}
}
