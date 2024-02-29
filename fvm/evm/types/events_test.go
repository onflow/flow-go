package types_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	cdcCommon "github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	flowSdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
)

type blockEventPayload struct {
	Height            uint64           `cadence:"height"`
	Hash              string           `cadence:"hash"`
	TotalSupply       cadence.Int      `cadence:"totalSupply"`
	ParentBlockHash   string           `cadence:"parentHash"`
	ReceiptRoot       string           `cadence:"receiptRoot"`
	TransactionHashes []cadence.String `cadence:"transactionHashes"`
}

type txEventPayload struct {
	BlockHeight             uint64 `cadence:"blockHeight"`
	BlockHash               string `cadence:"blockHash"`
	TransactionHash         string `cadence:"transactionHash"`
	Transaction             string `cadence:"transaction"`
	Failed                  bool   `cadence:"failed"`
	VMError                 string `cadence:"vmError"`
	TransactionType         uint8  `cadence:"transactionType"`
	GasConsumed             uint64 `cadence:"gasConsumed"`
	DeployedContractAddress string `cadence:"deployedContractAddress"`
	ReturnedValue           string `cadence:"returnedValue"`
	Logs                    string `cadence:"logs"`
}

func TestEVMBlockExecutedEventCCFEncodingDecoding(t *testing.T) {
	t.Parallel()

	block := &types.Block{
		Height:          2,
		TotalSupply:     big.NewInt(1500),
		ParentBlockHash: gethCommon.HexToHash("0x2813452cff514c3054ac9f40cd7ce1b016cc78ab7f99f1c6d49708837f6e06d1"),
		ReceiptRoot:     gethCommon.Hash{},
		TransactionHashes: []gethCommon.Hash{
			gethCommon.HexToHash("0x70b67ce6710355acf8d69b2ea013d34e212bc4824926c5d26f189c1ca9667246"),
		},
	}

	event := types.NewBlockExecutedEvent(block)
	ev, err := event.Payload.CadenceEvent()
	require.NoError(t, err)

	var bep blockEventPayload
	err = cadence.DecodeFields(ev, &bep)
	require.NoError(t, err)

	assert.Equal(t, bep.Height, block.Height)

	blockHash, err := block.Hash()
	require.NoError(t, err)
	assert.Equal(t, bep.Hash, blockHash.Hex())

	assert.Equal(t, bep.TotalSupply.Value, block.TotalSupply)
	assert.Equal(t, bep.ParentBlockHash, block.ParentBlockHash.Hex())
	assert.Equal(t, bep.ReceiptRoot, block.ReceiptRoot.Hex())

	hashes := make([]gethCommon.Hash, len(bep.TransactionHashes))
	for i, h := range bep.TransactionHashes {
		hashes[i] = gethCommon.HexToHash(h.ToGoValue().(string))
	}
	assert.Equal(t, hashes, block.TransactionHashes)

	v, err := ccf.Encode(ev)
	require.NoError(t, err)
	assert.Equal(t, ccf.HasMsgPrefix(v), true)

	evt, err := ccf.Decode(nil, v)
	require.NoError(t, err)

	assert.Equal(t, evt.Type().ID(), "evm.BlockExecuted")

	location, qualifiedIdentifier, err := cdcCommon.DecodeTypeID(nil, "evm.BlockExecuted")
	require.NoError(t, err)

	assert.Equal(t, flowSdk.EVMLocation{}, location)
	assert.Equal(t, "BlockExecuted", qualifiedIdentifier)
}

func TestEVMTransactionExecutedEventCCFEncodingDecoding(t *testing.T) {
	t.Parallel()

	txEncoded := "fff83b81ff0194000000000000000000000000000000000000000094000000000000000000000000000000000000000180895150ae84a8cdf00000825208"
	txBytes, err := hex.DecodeString(txEncoded)
	require.NoError(t, err)
	txHash := testutils.RandomCommonHash(t)
	blockHash := testutils.RandomCommonHash(t)
	data := "000000000000000000000000000000000000000000000000000000000000002a"
	dataBytes, err := hex.DecodeString(data)
	require.NoError(t, err)
	blockHeight := uint64(2)
	log := &gethTypes.Log{
		Index:       1,
		BlockNumber: blockHeight,
		BlockHash:   blockHash,
		TxHash:      txHash,
		TxIndex:     3,
		Address:     gethCommon.HexToAddress("0x99466ed2e37b892a2ee3e9cd55a98b68f5735db2"),
		Data:        dataBytes,
		Topics: []gethCommon.Hash{
			gethCommon.HexToHash("0x24abdb5865df5079dcc5ac590ff6f01d5c16edbc5fab4e195d9febd1114503da"),
		},
	}
	vmError := fmt.Errorf("ran out of gas")
	txResult := &types.Result{
		VMError:                 vmError,
		TxType:                  255,
		GasConsumed:             23200,
		DeployedContractAddress: types.NewAddress(gethCommon.HexToAddress("0x99466ed2e37b892a2ee3e9cd55a98b68f5735db2")),
		ReturnedValue:           dataBytes,
		Logs:                    []*gethTypes.Log{log},
	}

	t.Run("evm.TransactionExecuted with failed status", func(t *testing.T) {
		event := types.NewTransactionExecutedEvent(
			blockHeight,
			txBytes,
			blockHash,
			txHash,
			txResult,
		)
		ev, err := event.Payload.CadenceEvent()
		require.NoError(t, err)

		var tep txEventPayload
		err = cadence.DecodeFields(ev, &tep)
		require.NoError(t, err)

		assert.Equal(t, tep.BlockHeight, blockHeight)
		assert.Equal(t, tep.BlockHash, blockHash.Hex())
		assert.Equal(t, tep.TransactionHash, txHash.Hex())
		assert.Equal(t, tep.Transaction, txEncoded)
		assert.True(t, tep.Failed)
		assert.Equal(t, tep.VMError, vmError.Error())
		assert.Equal(t, tep.TransactionType, txResult.TxType)
		assert.Equal(t, tep.GasConsumed, txResult.GasConsumed)
		assert.Equal(
			t,
			tep.DeployedContractAddress,
			txResult.DeployedContractAddress.ToCommon().Hex(),
		)
		assert.Equal(t, tep.ReturnedValue, data)

		encodedLogs, err := rlp.EncodeToBytes(txResult.Logs)
		require.NoError(t, err)
		assert.Equal(t, tep.Logs, hex.EncodeToString(encodedLogs))

		v, err := ccf.Encode(ev)
		require.NoError(t, err)
		assert.Equal(t, ccf.HasMsgPrefix(v), true)

		evt, err := ccf.Decode(nil, v)
		require.NoError(t, err)

		assert.Equal(t, evt.Type().ID(), "evm.TransactionExecuted")

		location, qualifiedIdentifier, err := cdcCommon.DecodeTypeID(nil, "evm.TransactionExecuted")
		require.NoError(t, err)

		assert.Equal(t, flowSdk.EVMLocation{}, location)
		assert.Equal(t, "TransactionExecuted", qualifiedIdentifier)
	})

	t.Run("evm.TransactionExecuted with non-failed status", func(t *testing.T) {
		txResult.VMError = nil

		event := types.NewTransactionExecutedEvent(
			blockHeight,
			txBytes,
			blockHash,
			txHash,
			txResult,
		)
		ev, err := event.Payload.CadenceEvent()
		require.NoError(t, err)

		var tep txEventPayload
		err = cadence.DecodeFields(ev, &tep)
		require.NoError(t, err)

		assert.Equal(t, tep.BlockHeight, blockHeight)
		assert.Equal(t, tep.BlockHash, blockHash.Hex())
		assert.Equal(t, tep.TransactionHash, txHash.Hex())
		assert.Equal(t, tep.Transaction, txEncoded)
		assert.False(t, tep.Failed)
		assert.Equal(t, "", tep.VMError)
		assert.Equal(t, tep.TransactionType, txResult.TxType)
		assert.Equal(t, tep.GasConsumed, txResult.GasConsumed)
		assert.Equal(
			t,
			tep.DeployedContractAddress,
			txResult.DeployedContractAddress.ToCommon().Hex(),
		)
		assert.Equal(t, tep.ReturnedValue, data)

		encodedLogs, err := rlp.EncodeToBytes(txResult.Logs)
		require.NoError(t, err)
		assert.Equal(t, tep.Logs, hex.EncodeToString(encodedLogs))

		v, err := ccf.Encode(ev)
		require.NoError(t, err)
		assert.Equal(t, ccf.HasMsgPrefix(v), true)

		evt, err := ccf.Decode(nil, v)
		require.NoError(t, err)

		assert.Equal(t, evt.Type().ID(), "evm.TransactionExecuted")

		location, qualifiedIdentifier, err := cdcCommon.DecodeTypeID(nil, "evm.TransactionExecuted")
		require.NoError(t, err)

		assert.Equal(t, flowSdk.EVMLocation{}, location)
		assert.Equal(t, "TransactionExecuted", qualifiedIdentifier)
	})
}
