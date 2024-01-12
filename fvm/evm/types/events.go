package types

import (
	"encoding/hex"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/model/flow"
)

const (
	EventTypeBlockExecuted       flow.EventType = "evm.BlockExecuted"
	EventTypeTransactionExecuted flow.EventType = "evm.TransactionExecuted"
)

type EventPayload interface {
	// CadenceEvent creates a Cadence event type
	CadenceEvent() (cadence.Event, error)
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

var transactionExecutedEventCadenceType = &cadence.EventType{
	Location:            stdlib.FlowLocation{},
	QualifiedIdentifier: string(EventTypeTransactionExecuted),
	Fields: []cadence.Field{
		cadence.NewField("blockHeight", cadence.UInt64Type{}),
		cadence.NewField("transactionHash", cadence.StringType{}),
		cadence.NewField("transaction", cadence.StringType{}),
		cadence.NewField("failed", cadence.BoolType{}),
		cadence.NewField("transactionType", cadence.UInt8Type{}),
		cadence.NewField("gasConsumed", cadence.UInt64Type{}),
		cadence.NewField("deployedContractAddress", cadence.StringType{}),
		cadence.NewField("returnedValue", cadence.StringType{}),
		cadence.NewField("logs", cadence.StringType{}),
	},
}

// todo add decoder for events from cadence to evm payload

func (p *TransactionExecutedPayload) CadenceEvent() (cadence.Event, error) {
	var encodedLogs []byte
	var err error
	if len(p.Result.Logs) > 0 {
		encodedLogs, err = rlp.EncodeToBytes(p.Result.Logs)
		if err != nil {
			return cadence.Event{}, err
		}
	}

	fields := []cadence.Value{
		cadence.NewUInt64(p.BlockHeight),
		cadence.String(p.TxHash.String()),
		cadence.String(hex.EncodeToString(p.TxEncoded)),
		cadence.NewBool(p.Result.Failed),
		cadence.NewUInt8(p.Result.TxType),
		cadence.NewUInt64(p.Result.GasConsumed),
		cadence.String(hex.EncodeToString(p.Result.DeployedContractAddress.Bytes())),
		cadence.String(hex.EncodeToString(p.Result.ReturnedValue)),
		cadence.String(hex.EncodeToString(encodedLogs)),
	}

	return cadence.
		NewEvent(fields).
		WithType(transactionExecutedEventCadenceType), nil
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

var blockExecutedEventCadenceType = &cadence.EventType{
	Location:            stdlib.FlowLocation{}, // todo create evm custom location
	QualifiedIdentifier: string(EventTypeBlockExecuted),
	Fields: []cadence.Field{
		cadence.NewField("height", cadence.UInt64Type{}),
		cadence.NewField("totalSupply", cadence.UInt64Type{}),
		cadence.NewField("parentHash", cadence.StringType{}),
		cadence.NewField("receiptRoot", cadence.StringType{}),
		cadence.NewField(
			"transactionHashes",
			cadence.NewVariableSizedArrayType(cadence.StringType{}),
		),
	},
}

type BlockExecutedEventPayload struct {
	Block *Block
}

func (p *BlockExecutedEventPayload) CadenceEvent() (cadence.Event, error) {
	hashes := make([]cadence.Value, len(p.Block.TransactionHashes))
	for i, hash := range p.Block.TransactionHashes {
		hashes[i] = cadence.String(hash.String())
	}

	fields := []cadence.Value{
		cadence.NewUInt64(p.Block.Height),
		cadence.NewUInt64(p.Block.TotalSupply),
		cadence.String(p.Block.ParentBlockHash.String()),
		cadence.String(p.Block.ReceiptRoot.String()),
		cadence.NewArray(hashes).WithType(cadence.NewVariableSizedArrayType(cadence.StringType{})),
	}

	return cadence.
		NewEvent(fields).
		WithType(blockExecutedEventCadenceType), nil
}

func NewBlockExecutedEvent(block *Block) *Event {
	return &Event{
		Etype: EventTypeBlockExecuted,
		Payload: &BlockExecutedEventPayload{
			Block: block,
		},
	}
}
