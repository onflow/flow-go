package types

import (
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/onflow/flow-go/model/flow"
)

const (
	EventTypeBlockExecuted       flow.EventType = "evm.BlockExecuted"
	EventTypeTransactionExecuted flow.EventType = "evm.TransactionExecuted"
)

type EventPayload interface {
	Encode() ([]byte, error)
}

type Event struct {
	Etype   flow.EventType
	Payload EventPayload
}

// we might break this event into two (tx included /tx executed) if size becomes an issue
type TransactionExecutedPayload struct {
	BlockHeight uint64
	TxEncoded   []byte
	TxHash      gethCommon.Hash
	Result      *Result
}

func (p *TransactionExecutedPayload) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(p)
}

func NewTransactionExecutedEvent(
	height uint64,
	txEncoded []byte,
	txHash gethCommon.Hash,
	result *Result,
) *Event {
	return &Event{
		Etype: EventTypeTransactionExecuted,
		Payload: &TransactionExecutedPayload{
			BlockHeight: height,
			TxEncoded:   txEncoded,
			TxHash:      txHash,
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
