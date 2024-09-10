package events

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/rlp"

	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const (
	EventTypeBlockExecuted       flow.EventType = "EVM.BlockExecuted"
	EventTypeTransactionExecuted flow.EventType = "EVM.TransactionExecuted"
)

type EventPayload interface {
	// ToCadence converts the event to Cadence event
	ToCadence(chainID flow.ChainID) (cadence.Event, error)
}

type Event struct {
	Etype   flow.EventType
	Payload EventPayload
}

// todo we might have to break this event into two (tx included /tx executed) if size becomes an issue

type transactionEvent struct {
	Payload     []byte        // transaction RLP-encoded payload
	Result      *types.Result // transaction execution result
	BlockHeight uint64
}

// NewTransactionEvent creates a new transaction event with the given parameters
// - result: the result of the transaction execution
// - payload: the RLP-encoded payload of the transaction
// - blockHeight: the height of the block where the transaction is included
func NewTransactionEvent(
	result *types.Result,
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

func (p *transactionEvent) ToCadence(chainID flow.ChainID) (cadence.Event, error) {
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

	eventType := stdlib.CadenceTypesForChain(chainID).TransactionExecuted

	// the first 4 bytes of StateChangeCommitment is used as checksum
	var checksum [4]byte
	if len(p.Result.StateChangeCommitment) >= ChecksumLength {
		copy(checksum[:ChecksumLength], p.Result.StateChangeCommitment[:ChecksumLength])
	}
	return cadence.NewEvent([]cadence.Value{
		hashToCadenceArrayValue(p.Result.TxHash),
		cadence.NewUInt16(p.Result.Index),
		cadence.NewUInt8(p.Result.TxType),
		bytesToCadenceUInt8ArrayValue(p.Payload),
		cadence.NewUInt16(uint16(p.Result.ResultSummary().ErrorCode)),
		cadence.String(errorMsg),
		cadence.NewUInt64(p.Result.GasConsumed),
		deployedAddress,
		bytesToCadenceUInt8ArrayValue(encodedLogs),
		cadence.NewUInt64(p.BlockHeight),
		bytesToCadenceUInt8ArrayValue(p.Result.ReturnedData),
		bytesToCadenceUInt8ArrayValue(p.Result.PrecompiledCalls),
		checksumToCadenceArrayValue(checksum),
	}).WithType(eventType), nil
}

type blockEvent struct {
	*types.Block
}

// NewBlockEvent creates a new block event with the given block as payload.
func NewBlockEvent(block *types.Block) *Event {
	return &Event{
		Etype:   EventTypeBlockExecuted,
		Payload: &blockEvent{block},
	}
}

func (p *blockEvent) ToCadence(chainID flow.ChainID) (cadence.Event, error) {
	blockHash, err := p.Hash()
	if err != nil {
		return cadence.Event{}, err
	}

	eventType := stdlib.CadenceTypesForChain(chainID).BlockExecuted

	return cadence.NewEvent([]cadence.Value{
		cadence.NewUInt64(p.Height),
		hashToCadenceArrayValue(blockHash),
		cadence.NewUInt64(p.Timestamp),
		cadence.NewIntFromBig(p.TotalSupply),
		cadence.NewUInt64(p.TotalGasUsed),
		hashToCadenceArrayValue(p.ParentBlockHash),
		hashToCadenceArrayValue(p.ReceiptRoot),
		hashToCadenceArrayValue(p.TransactionHashRoot),
		hashToCadenceArrayValue(p.PrevRandao),
	}).WithType(eventType), nil
}

type BlockEventPayload struct {
	Height              uint64          `cadence:"height"`
	Hash                gethCommon.Hash `cadence:"hash"`
	Timestamp           uint64          `cadence:"timestamp"`
	TotalSupply         cadence.Int     `cadence:"totalSupply"`
	TotalGasUsed        uint64          `cadence:"totalGasUsed"`
	ParentBlockHash     gethCommon.Hash `cadence:"parentHash"`
	ReceiptRoot         gethCommon.Hash `cadence:"receiptRoot"`
	TransactionHashRoot gethCommon.Hash `cadence:"transactionHashRoot"`
	PrevRandao          gethCommon.Hash `cadence:"prevrandao"`
}

// blockEventPayloadV0 legacy format of the block without prevrando field
type blockEventPayloadV0 struct {
	Height              uint64          `cadence:"height"`
	Hash                gethCommon.Hash `cadence:"hash"`
	Timestamp           uint64          `cadence:"timestamp"`
	TotalSupply         cadence.Int     `cadence:"totalSupply"`
	TotalGasUsed        uint64          `cadence:"totalGasUsed"`
	ParentBlockHash     gethCommon.Hash `cadence:"parentHash"`
	ReceiptRoot         gethCommon.Hash `cadence:"receiptRoot"`
	TransactionHashRoot gethCommon.Hash `cadence:"transactionHashRoot"`
}

// decodeLegacyBlockEventPayload decodes any legacy block formats into
// current version of the block event payload.
func decodeLegacyBlockEventPayload(event cadence.Event) (*BlockEventPayload, error) {
	var lb blockEventPayloadV0
	if err := cadence.DecodeFields(event, &lb); err != nil {
		return nil, err
	}

	return &BlockEventPayload{
		Height:              lb.Height,
		Hash:                lb.Hash,
		Timestamp:           lb.Timestamp,
		TotalSupply:         lb.TotalSupply,
		TotalGasUsed:        lb.TotalGasUsed,
		ParentBlockHash:     lb.ParentBlockHash,
		ReceiptRoot:         lb.ReceiptRoot,
		TransactionHashRoot: lb.TransactionHashRoot,
	}, nil
}

// DecodeBlockEventPayload decodes Cadence event into block event payload.
func DecodeBlockEventPayload(event cadence.Event) (*BlockEventPayload, error) {
	var block BlockEventPayload
	if err := cadence.DecodeFields(event, &block); err != nil {
		if block, err := decodeLegacyBlockEventPayload(event); err == nil {
			return block, nil
		}
		return nil, err
	}
	return &block, nil
}

type TransactionEventPayload struct {
	Hash                gethCommon.Hash `cadence:"hash"`
	Index               uint16          `cadence:"index"`
	TransactionType     uint8           `cadence:"type"`
	Payload             []byte          `cadence:"payload"`
	ErrorCode           uint16          `cadence:"errorCode"`
	GasConsumed         uint64          `cadence:"gasConsumed"`
	ContractAddress     string          `cadence:"contractAddress"`
	Logs                []byte          `cadence:"logs"`
	BlockHeight         uint64          `cadence:"blockHeight"`
	ErrorMessage        string          `cadence:"errorMessage"`
	ReturnedData        []byte          `cadence:"returnedData"`
	PrecompiledCalls    []byte          `cadence:"precompiledCalls"`
	StateUpdateChecksum [4]byte         `cadence:"stateUpdateChecksum"`
}

// transactionEventPayloadV0 legacy format of the transaction event without stateUpdateChecksum field
type transactionEventPayloadV0 struct {
	Hash             gethCommon.Hash `cadence:"hash"`
	Index            uint16          `cadence:"index"`
	TransactionType  uint8           `cadence:"type"`
	Payload          []byte          `cadence:"payload"`
	ErrorCode        uint16          `cadence:"errorCode"`
	GasConsumed      uint64          `cadence:"gasConsumed"`
	ContractAddress  string          `cadence:"contractAddress"`
	Logs             []byte          `cadence:"logs"`
	BlockHeight      uint64          `cadence:"blockHeight"`
	ErrorMessage     string          `cadence:"errorMessage"`
	ReturnedData     []byte          `cadence:"returnedData"`
	PrecompiledCalls []byte          `cadence:"precompiledCalls"`
}

// decodeLegacyTransactionEventPayload decodes any legacy transaction formats into
// current version of the transaction event payload.
func decodeLegacyTransactionEventPayload(event cadence.Event) (*TransactionEventPayload, error) {
	var tx transactionEventPayloadV0
	if err := cadence.DecodeFields(event, &tx); err != nil {
		return nil, err
	}
	return &TransactionEventPayload{
		Hash:             tx.Hash,
		Index:            tx.Index,
		TransactionType:  tx.TransactionType,
		Payload:          tx.Payload,
		ErrorCode:        tx.ErrorCode,
		GasConsumed:      tx.GasConsumed,
		ContractAddress:  tx.ContractAddress,
		Logs:             tx.Logs,
		BlockHeight:      tx.BlockHeight,
		ErrorMessage:     tx.ErrorMessage,
		ReturnedData:     tx.ReturnedData,
		PrecompiledCalls: tx.PrecompiledCalls,
	}, nil
}

// DecodeTransactionEventPayload decodes Cadence event into transaction event payload.
func DecodeTransactionEventPayload(event cadence.Event) (*TransactionEventPayload, error) {
	var tx TransactionEventPayload
	if err := cadence.DecodeFields(event, &tx); err != nil {
		if legTx, err := decodeLegacyTransactionEventPayload(event); err == nil {
			return legTx, nil
		}
		return nil, err
	}
	return &tx, nil

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
