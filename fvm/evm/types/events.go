package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime/common"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/model/flow"
)

const (
	EventTypeBlockExecuted       flow.EventType = "evm.BlockExecuted"
	EventTypeTransactionExecuted flow.EventType = "evm.TransactionExecuted"
	evmLocationPrefix                           = "evm"
	locationDivider                             = "."
)

type EventPayload interface {
	// CadenceEvent creates a Cadence event type
	CadenceEvent() (cadence.Event, error)
}

type Event struct {
	Etype   flow.EventType
	Payload EventPayload
}

var _ common.Location = EVMLocation{}

type EVMLocation struct{}

func (l EVMLocation) TypeID(memoryGauge common.MemoryGauge, qualifiedIdentifier string) common.TypeID {
	id := fmt.Sprintf("%s.%s", locationDivider, qualifiedIdentifier)
	common.UseMemory(memoryGauge, common.NewRawStringMemoryUsage(len(id)))

	return common.TypeID(id)
}

func (l EVMLocation) QualifiedIdentifier(typeID common.TypeID) string {
	pieces := strings.SplitN(string(typeID), locationDivider, 2)

	if len(pieces) < 2 {
		return ""
	}

	return pieces[1]
}

func (l EVMLocation) String() string {
	return evmLocationPrefix
}

func (l EVMLocation) Description() string {
	return evmLocationPrefix
}

func (l EVMLocation) ID() string {
	return evmLocationPrefix
}

func (l EVMLocation) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type string
	}{
		Type: "EVMLocation",
	})
}

// we might break this event into two (tx included /tx executed) if size becomes an issue
type TransactionExecutedPayload struct {
	BlockHeight uint64
	TxEncoded   []byte
	TxHash      gethCommon.Hash
	Result      *Result
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
			EVMLocation{},
			string(EventTypeTransactionExecuted),
			[]cadence.Field{
				cadence.NewField("blockHeight", cadence.UInt64Type{}),
				cadence.NewField("transactionHash", cadence.StringType{}),
				cadence.NewField("transaction", cadence.StringType{}),
				cadence.NewField("failed", cadence.BoolType{}),
				cadence.NewField("transactionType", cadence.UInt8Type{}),
				cadence.NewField("gasConsumed", cadence.UInt64Type{}),
				cadence.NewField("stateRootHash", cadence.StringType{}),
				cadence.NewField("deployedContractAddress", cadence.StringType{}),
				cadence.NewField("returnedValue", cadence.StringType{}),
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
		Location:            EVMLocation{},
		QualifiedIdentifier: string(EventTypeBlockExecuted),
		Fields: []cadence.Field{
			cadence.NewField("height", cadence.UInt64Type{}),
			cadence.NewField("totalSupply", cadence.UInt64Type{}),
			cadence.NewField("parentHash", cadence.StringType{}),
			cadence.NewField("stateRoot", cadence.StringType{}),
			cadence.NewField("receiptRoot", cadence.StringType{}),
			cadence.NewField(
				"transactionHashes",
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
