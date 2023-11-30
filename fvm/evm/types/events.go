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
	Encode() ([]byte, error)
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

func (p *TransactionExecutedPayload) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(p)
}

func (p *TransactionExecutedPayload) CadenceEvent() (cadence.Event, error) {
	var encodedLogs []byte
	var err error
	if len(p.Result.Logs) > 0 {
		encodedLogs, err = rlp.EncodeToBytes(p.Result.Logs)
		if err != nil {
			return cadence.Event{}, err
		}
	}

	return cadence.Event{
		EventType: cadence.NewEventType(
			stdlib.FlowLocation{},
			string(EventTypeTransactionExecuted),
			[]cadence.Field{
				cadence.NewField("block_height", cadence.UInt64Type{}),
				cadence.NewField("transaction_hash", cadence.StringType{}),
				cadence.NewField("transaction", cadence.StringType{}),
				cadence.NewField("failed", cadence.BoolType{}),
				cadence.NewField("transaction_type", cadence.UInt8Type{}),
				cadence.NewField("gas_consumed", cadence.UInt64Type{}),
				cadence.NewField("state_root_hash", cadence.StringType{}),
				cadence.NewField("deployed_contract_address", cadence.StringType{}),
				cadence.NewField("returned_value", cadence.StringType{}),
				cadence.NewField("logs", cadence.StringType{}),
			},
			nil,
		),
		Fields: []cadence.Value{
			cadence.NewUInt64(p.BlockHeight),
			cadence.String(p.TxHash.String()),
			cadence.String(hex.EncodeToString(p.TxEncoded)),
			cadence.NewBool(p.Result.Failed),
			cadence.NewUInt8(p.Result.TxType),
			cadence.NewUInt64(p.Result.GasConsumed),
			cadence.String(p.Result.StateRootHash.String()),
			cadence.String(hex.EncodeToString(p.Result.DeployedContractAddress.Bytes())),
			cadence.String(hex.EncodeToString(p.Result.ReturnedValue)),
			cadence.String(hex.EncodeToString(encodedLogs)),
		},
	}, nil
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

func (p *BlockExecutedEventPayload) CadenceEvent() (cadence.Event, error) {
	hashesType := cadence.NewConstantSizedArrayType(uint(len(p.Block.TransactionHashes)), cadence.StringType{})
	hashes := make([]cadence.Value, len(p.Block.TransactionHashes))
	for i, hash := range p.Block.TransactionHashes {
		hashes[i] = cadence.String(hash.String())
	}

	return cadence.NewEvent([]cadence.Value{
		cadence.NewUInt64(p.Block.Height),
		cadence.NewUInt64(p.Block.TotalSupply),
		cadence.String(p.Block.ReceiptRoot.String()),
		cadence.String(p.Block.ParentBlockHash.String()),
		cadence.String(p.Block.StateRoot.String()),
		cadence.NewArray(hashes).WithType(hashesType),
	}).WithType(&cadence.EventType{
		Location:            stdlib.FlowLocation{}, // todo create evm custom location
		QualifiedIdentifier: string(EventTypeBlockExecuted),
		Fields: []cadence.Field{
			cadence.NewField("height", cadence.UInt64Type{}),
			cadence.NewField("total_supply", cadence.UInt64Type{}),
			cadence.NewField("parent_hash", cadence.StringType{}),
			cadence.NewField("state_root", cadence.StringType{}),
			cadence.NewField("receipt_root", cadence.StringType{}),
			cadence.NewField(
				"transaction_hashes",
				hashesType,
			),
		},
	}), nil
}

func NewBlockExecutedEvent(block *Block) *Event {
	return &Event{
		Etype: EventTypeBlockExecuted,
		Payload: &BlockExecutedEventPayload{
			Block: block,
		},
	}
}
