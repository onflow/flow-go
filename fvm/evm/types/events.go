package types

import (
	"fmt"

	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/runtime/common"
	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/rlp"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/flow"
)

const (
	EventTypeBlockExecuted       flow.EventType = "EVM.BlockExecuted"
	EventTypeTransactionExecuted flow.EventType = "EVM.TransactionExecuted"
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
) *Event {
	return &Event{
		Etype: EventTypeTransactionExecuted,
		Payload: &transactionEvent{
			BlockHeight: blockHeight,
			Payload:     payload,
			Result:      result,
		},
	}
}

var transactionEventFields = []cadence.Field{
	cadence.NewField("hash", cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
	cadence.NewField("index", cadence.UInt16Type),
	cadence.NewField("type", cadence.UInt8Type),
	cadence.NewField("payload", cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
	cadence.NewField("errorCode", cadence.UInt16Type),
	cadence.NewField("errorMessage", cadence.StringType),
	cadence.NewField("gasConsumed", cadence.UInt64Type),
	cadence.NewField("contractAddress", cadence.StringType),
	cadence.NewField("logs", cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
	cadence.NewField("blockHeight", cadence.UInt64Type),
	// todo we can remove hash and just reference block by height (evm-gateway dependency)
	cadence.NewField("blockHash", cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
	cadence.NewField("returnedData", cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
	cadence.NewField("precompiledCalls", cadence.NewVariableSizedArrayType(cadence.UInt8Type)),
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

	errorMsg := ""
	if p.Result.VMError != nil {
		errorMsg = p.Result.VMError.Error()
	}
	// both error would never happen at the same time
	// but in case the priority is by validation error
	if p.Result.ValidationError != nil {
		errorMsg = p.Result.ValidationError.Error()
	}

	eventType := cadence.NewEventType(
		location,
		string(EventTypeTransactionExecuted),
		transactionEventFields,
		nil,
	)

	return cadence.NewEvent([]cadence.Value{
		BytesToCadenceUInt8ArrayValue(p.Result.TxHash.Bytes()),
		cadence.NewUInt16(p.Result.Index),
		cadence.NewUInt8(p.Result.TxType),
		BytesToCadenceUInt8ArrayValue(p.Payload),
		cadence.NewUInt16(uint16(p.Result.ResultSummary().ErrorCode)),
		cadence.String(errorMsg),
		cadence.NewUInt64(p.Result.GasConsumed),
		deployedAddress,
		BytesToCadenceUInt8ArrayValue(encodedLogs),
		cadence.NewUInt64(p.BlockHeight),
		BytesToCadenceUInt8ArrayValue(p.BlockHash.Bytes()),
		BytesToCadenceUInt8ArrayValue(p.Result.ReturnedData),
		BytesToCadenceUInt8ArrayValue(p.Result.PrecompiledCalls),
	}).WithType(eventType), nil
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

var blockEventFields = []cadence.Field{
	cadence.NewField("height", cadence.UInt64Type),
	cadence.NewField("hash",
		cadence.NewVariableSizedArrayType(cadence.UInt8Type),
	),
	cadence.NewField("timestamp", cadence.UInt64Type),
	cadence.NewField("totalSupply", cadence.IntType),
	cadence.NewField("totalGasUsed", cadence.UInt64Type),
	cadence.NewField(
		"parentHash",
		cadence.NewVariableSizedArrayType(cadence.UInt8Type),
	),
	cadence.NewField(
		"receiptRoot",
		cadence.NewVariableSizedArrayType(cadence.UInt8Type),
	),
	cadence.NewField(
		"transactionHashes",
		cadence.NewVariableSizedArrayType(
			cadence.NewVariableSizedArrayType(cadence.UInt8Type),
		),
	),
}

func (p *blockEvent) ToCadence(location common.Location) (cadence.Event, error) {

	transactionHashes := make([]cadence.Value, len(p.TransactionHashes))
	for i, hash := range p.TransactionHashes {
		transactionHashes[i] = BytesToCadenceUInt8ArrayValue(hash.Bytes())
	}

	blockHash, err := p.Hash()
	if err != nil {
		return cadence.Event{}, err
	}

	eventType := cadence.NewEventType(
		location,
		string(EventTypeBlockExecuted),
		blockEventFields,
		nil,
	)

	return cadence.NewEvent([]cadence.Value{
		cadence.NewUInt64(p.Height),
		BytesToCadenceUInt8ArrayValue(blockHash.Bytes()),
		cadence.NewUInt64(p.Timestamp),
		cadence.NewIntFromBig(p.TotalSupply),
		cadence.NewUInt64(p.TotalGasUsed),
		BytesToCadenceUInt8ArrayValue(p.ParentBlockHash.Bytes()),
		BytesToCadenceUInt8ArrayValue(p.ReceiptRoot.Bytes()),
		cadence.NewArray(transactionHashes).
			WithType(
				cadence.NewVariableSizedArrayType(
					cadence.NewVariableSizedArrayType(cadence.UInt8Type),
				),
			),
	}).WithType(eventType), nil
}

type BlockEventPayload struct {
	Height            uint64      `cadence:"height"`
	Hash              []byte      `cadence:"hash"`
	Timestamp         uint64      `cadence:"timestamp"`
	TotalSupply       cadence.Int `cadence:"totalSupply"`
	TotalGasUsed      uint64      `cadence:"totalGasUsed"`
	ParentBlockHash   []byte      `cadence:"parentHash"`
	ReceiptRoot       []byte      `cadence:"receiptRoot"`
	TransactionHashes [][]byte    `cadence:"transactionHashes"`
}

// DecodeBlockEventPayload decodes Cadence event into block event payload.
func DecodeBlockEventPayload(event cadence.Event) (*BlockEventPayload, error) {
	var block BlockEventPayload
	err := cadence.DecodeFields(event, &block)
	return &block, err
}

type TransactionEventPayload struct {
	Hash             []byte `cadence:"hash"`
	Index            uint16 `cadence:"index"`
	TransactionType  uint8  `cadence:"type"`
	Payload          []byte `cadence:"payload"`
	ErrorCode        uint16 `cadence:"errorCode"`
	GasConsumed      uint64 `cadence:"gasConsumed"`
	ContractAddress  string `cadence:"contractAddress"`
	Logs             []byte `cadence:"logs"`
	BlockHeight      uint64 `cadence:"blockHeight"`
	ErrorMessage     string `cadence:"errorMessage"`
	ReturnedData     []byte `cadence:"returnedData"`
	PrecompiledCalls []byte `cadence:"precompiledCalls"`
}

// DecodeTransactionEventPayload decodes Cadence event into transaction event payload.
func DecodeTransactionEventPayload(event cadence.Event) (*TransactionEventPayload, error) {
	var tx TransactionEventPayload
	err := cadence.DecodeFields(event, &tx)
	return &tx, err
}

// FLOWTokensDepositedEventPayload captures payloads for a FlowTokenDeposited event
type FLOWTokensDepositedEventPayload struct {
	Address                string         `cadence:"address"`
	Amount                 cadence.UFix64 `cadence:"amount"`
	DepositedUUID          uint64         `cadence:"depositedUUID"`
	BalanceAfterInAttoFlow cadence.Int    `cadence:"balanceAfterInAttoFlow"`
}

// DecodeFLOWTokensDepositedEventPayload decodes a flow FLOWTokenDeposited
// events into a FLOWTokensEventDepositedPayload
func DecodeFLOWTokensDepositedEventPayload(event cadence.Event) (*FLOWTokensDepositedEventPayload, error) {
	var payload FLOWTokensDepositedEventPayload
	err := cadence.DecodeFields(event, &payload)
	return &payload, err
}

// FLOWTokensWithdrawnEventPayload captures payloads for a FlowTokensWithdrawn event
type FLOWTokensWithdrawnEventPayload struct {
	Address                string         `cadence:"address"`
	Amount                 cadence.UFix64 `cadence:"amount"`
	WithdrawnUUID          uint64         `cadence:"withdrawnUUID"`
	BalanceAfterInAttoFlow cadence.Int    `cadence:"balanceAfterInAttoFlow"`
}

// DecodeFLOWTokensWithdrawnEventPayload decodes a flow FLOWTokensWithdrawn
// events into a FLOWTokensEventDepositedPayload
func DecodeFLOWTokensWithdrawnEventPayload(event cadence.Event) (*FLOWTokensWithdrawnEventPayload, error) {
	var payload FLOWTokensWithdrawnEventPayload
	err := cadence.DecodeFields(event, &payload)
	return &payload, err
}

func FlowEventToCadenceEvent(event flow.Event) (cadence.Event, error) {
	ev, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return cadence.Event{}, err
	}

	cadenceEvent, ok := ev.(cadence.Event)
	if !ok {
		return cadence.Event{}, fmt.Errorf("event can not be casted to a cadence event")
	}
	return cadenceEvent, nil
}
