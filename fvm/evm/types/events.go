package types

import (
	"encoding/hex"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/cadence"
	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/rlp"

	"github.com/onflow/flow-go/model/flow"
)

const (
	EventTypeBlockExecuted       flow.EventType = "BlockExecuted"
	EventTypeTransactionExecuted flow.EventType = "TransactionExecuted"
)

type EventPayload interface {
	// ToCadence converts the event to Cadence event
	ToCadence(location common.Location) (cadence.Event, error)
}

type Event struct {
	Etype   flow.EventType
	Payload EventPayload
}

// todo we might have to break this event into two (tx included /tx executed) if size becomes an issue

type transactionEvent struct {
	Payload     []byte  // transaction RLP-encoded payload
	Result      *Result // transaction execution result
	BlockHeight uint64
	BlockHash   gethCommon.Hash
}

// NewTransactionEvent creates a new transaction event with the given parameters
// - result: the result of the transaction execution
// - payload: the RLP-encoded payload of the transaction
// - blockHeight: the height of the block where the transaction is included
// - blockHash: the hash of the block where the transaction is included
func NewTransactionEvent(
	result *Result,
	payload []byte,
	blockHeight uint64,
	blockHash gethCommon.Hash,
) *Event {
	return &Event{
		Etype: EventTypeTransactionExecuted,
		Payload: &transactionEvent{
			BlockHeight: blockHeight,
			BlockHash:   blockHash,
			Payload:     payload,
			Result:      result,
		},
	}
}

func (p *transactionEvent) ToCadence(location common.Location) (cadence.Event, error) {
	var encodedLogs []byte
	var err error
	if len(p.Result.Logs) > 0 {
		encodedLogs, err = rlp.EncodeToBytes(p.Result.Logs)
		if err != nil {
			return cadence.Event{}, err
		}
	}

	deployedAddress := cadence.String("")
	if p.Result.DeployedContractAddress != nil {
		deployedAddress = cadence.String(p.Result.DeployedContractAddress.String())
	}

	return cadence.Event{
		EventType: cadence.NewEventType(
			location,
			string(EventTypeTransactionExecuted),
			[]cadence.Field{
				cadence.NewField("hash", cadence.StringType{}),
				cadence.NewField("index", cadence.UInt16Type{}),
				cadence.NewField("type", cadence.UInt8Type{}),
				cadence.NewField("payload", cadence.StringType{}),
				cadence.NewField("errorCode", cadence.UInt16Type{}),
				cadence.NewField("gasConsumed", cadence.UInt64Type{}),
				cadence.NewField("contractAddress", cadence.StringType{}),
				cadence.NewField("logs", cadence.StringType{}),
				cadence.NewField("blockHeight", cadence.UInt64Type{}),
				// todo we can remove hash and just reference block by height (evm-gateway dependency)
				cadence.NewField("blockHash", cadence.StringType{}),
			},
			nil,
		),
		Fields: []cadence.Value{
			cadence.String(p.Result.TxHash.String()),
			cadence.NewUInt16(p.Result.Index),
			cadence.NewUInt8(p.Result.TxType),
			cadence.String(hex.EncodeToString(p.Payload)),
			cadence.NewUInt16(uint16(p.Result.ResultSummary().ErrorCode)),
			cadence.NewUInt64(p.Result.GasConsumed),
			deployedAddress,
			cadence.String(hex.EncodeToString(encodedLogs)),
			cadence.NewUInt64(p.BlockHeight),
			cadence.String(p.BlockHash.String()),
		},
	}, nil
}

type blockEvent struct {
	*Block
}

// NewBlockEvent creates a new block event with the given block as payload.
func NewBlockEvent(block *Block) *Event {
	return &Event{
		Etype:   EventTypeBlockExecuted,
		Payload: &blockEvent{block},
	}
}

func (p *blockEvent) ToCadence(location common.Location) (cadence.Event, error) {
	hashes := make([]cadence.Value, len(p.TransactionHashes))
	for i, hash := range p.TransactionHashes {
		hashes[i] = cadence.String(hash.String())
	}

	blockHash, err := p.Hash()
	if err != nil {
		return cadence.Event{}, err
	}

	return cadence.Event{
		EventType: cadence.NewEventType(
			location,
			string(EventTypeBlockExecuted),
			[]cadence.Field{
				cadence.NewField("height", cadence.UInt64Type{}),
				cadence.NewField("hash", cadence.StringType{}),
				cadence.NewField("timestamp", cadence.UInt64Type{}),
				cadence.NewField("totalSupply", cadence.IntType{}),
				cadence.NewField("totalGasUsed", cadence.UInt64Type{}),
				cadence.NewField("parentHash", cadence.StringType{}),
				cadence.NewField("receiptRoot", cadence.StringType{}),
				cadence.NewField(
					"transactionHashes",
					cadence.NewVariableSizedArrayType(cadence.StringType{}),
				),
			},
			nil,
		),
		Fields: []cadence.Value{
			cadence.NewUInt64(p.Height),
			cadence.String(blockHash.String()),
			cadence.NewUInt64(p.Timestamp),
			cadence.NewIntFromBig(p.TotalSupply),
			cadence.NewUInt64(p.TotalGasUsed),
			cadence.String(p.ParentBlockHash.String()),
			cadence.String(p.ReceiptRoot.String()),
			cadence.NewArray(hashes).WithType(cadence.NewVariableSizedArrayType(cadence.StringType{})),
		},
	}, nil
}

type BlockEventPayload struct {
	Height            uint64           `cadence:"height"`
	Hash              string           `cadence:"hash"`
	Timestamp         uint64           `cadence:"timestamp"`
	TotalSupply       cadence.Int      `cadence:"totalSupply"`
	TotalGasUsed      uint64           `cadence:"totalGasUsed"`
	ParentBlockHash   string           `cadence:"parentHash"`
	ReceiptRoot       string           `cadence:"receiptRoot"`
	TransactionHashes []cadence.String `cadence:"transactionHashes"`
}

// DecodeBlockEventPayload decodes Cadence event into block event payload.
func DecodeBlockEventPayload(event cadence.Event) (*BlockEventPayload, error) {
	var block BlockEventPayload
	err := cadence.DecodeFields(event, &block)
	return &block, err
}

type TransactionEventPayload struct {
	Hash            string `cadence:"hash"`
	Index           uint16 `cadence:"index"`
	TransactionType uint8  `cadence:"type"`
	Payload         string `cadence:"payload"`
	ErrorCode       uint16 `cadence:"errorCode"`
	GasConsumed     uint64 `cadence:"gasConsumed"`
	ContractAddress string `cadence:"contractAddress"`
	Logs            string `cadence:"logs"`
	BlockHeight     uint64 `cadence:"blockHeight"`
	BlockHash       string `cadence:"blockHash"`
}

// DecodeTransactionEventPayload decodes Cadence event into transaction event payload.
func DecodeTransactionEventPayload(event cadence.Event) (*TransactionEventPayload, error) {
	var tx TransactionEventPayload
	err := cadence.DecodeFields(event, &tx)
	return &tx, err
}
