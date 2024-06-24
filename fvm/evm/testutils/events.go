package testutils

import (
	"encoding/hex"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	"github.com/onflow/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type BlockEventValidator struct {
	t       testing.TB
	payload *types.BlockEventPayload
}

func (bv *BlockEventValidator) HasHeight(height int) *BlockEventValidator {
	require.Equal(bv.t, height, int(bv.payload.Height))
	return bv
}

func (bv *BlockEventValidator) HasTransactionHashes(txHashes []string) *BlockEventValidator {
	require.Equal(bv.t, len(bv.payload.TransactionHashes), len(txHashes))
	for i, txHash := range txHashes {
		require.Equal(bv.t, txHash, string(bv.payload.TransactionHashes[i]))
	}
	return bv
}

func CheckBlockExecutedEvent(
	t testing.TB,
	events []flow.Event,
	index int,
) *BlockEventValidator {
	event := events[index]
	assert.Equal(t, event.Type, types.EventTypeBlockExecuted)
	ev, err := jsoncdc.Decode(nil, event.Payload)
	require.NoError(t, err)
	cev, ok := ev.(cadence.Event)
	require.True(t, ok)
	// TODO: add cev.EventType check with location
	// require.Equal(t, cev.EventType, types.EventTypeBlockExecuted)

	payload, err := types.DecodeBlockEventPayload(cev)
	require.NoError(t, err)
	return &BlockEventValidator{t, payload}
}

type TransactionEventValidator struct {
	t       testing.TB
	payload *types.TransactionEventPayload
}

func (tx *TransactionEventValidator) MatchesResult(result *types.Result) *TransactionEventValidator {
	resSummary := result.ResultSummary()
	require.Equal(tx.t, result.TxHash.String(), tx.payload.Hash)
	require.Equal(tx.t, result.Index, tx.payload.Index)
	require.Equal(tx.t, result.TxType, tx.payload.TransactionType)
	require.Equal(tx.t, int(resSummary.ErrorCode), int(tx.payload.ErrorCode))
	require.Equal(tx.t, resSummary.ErrorMessage, tx.payload.ErrorMessage)
	require.Equal(tx.t, result.GasConsumed, tx.payload.GasConsumed)
	deployedAddress := ""
	if result.DeployedContractAddress != nil {
		deployedAddress = result.DeployedContractAddress.String()
	}
	require.Equal(tx.t, deployedAddress, tx.payload.ContractAddress)
	require.Equal(tx.t, hex.EncodeToString(result.ReturnedData), tx.payload.ReturnedData)
	tx.HasLogs(result.Logs)
	return tx
}

func (tx *TransactionEventValidator) HasBlockHash(blockHash string) *TransactionEventValidator {
	require.Equal(tx.t, blockHash, tx.payload.BlockHash)
	return tx
}

func (tx *TransactionEventValidator) HasBlockIndex(blockIndex int) *TransactionEventValidator {
	require.Equal(tx.t, blockIndex, int(tx.payload.Index))
	return tx
}

func (tx *TransactionEventValidator) HasLogs(expectedLogs []*gethTypes.Log) *TransactionEventValidator {
	encodedLogs, err := hex.DecodeString(tx.payload.Logs)
	require.NoError(tx.t, err)

	expectedEncoded, err := rlp.EncodeToBytes(expectedLogs)
	require.NoError(tx.t, err)

	require.Equal(tx.t, expectedEncoded, encodedLogs)
	return tx
}

func CheckTransactionExecutedEvent(
	t testing.TB,
	events []flow.Event,
	index int,
) *TransactionEventValidator {
	event := events[index]
	assert.Equal(t, event.Type, types.EventTypeTransactionExecuted)
	ev, err := jsoncdc.Decode(nil, event.Payload)
	require.NoError(t, err)
	cev, ok := ev.(cadence.Event)
	require.True(t, ok)
	// TODO add check for the location in the cev.EventType
	// require.Equal(t, cev.EventType, types.EventTypeTransactionExecuted)
	payload, err := types.DecodeTransactionEventPayload(cev)
	require.NoError(t, err)
	return &TransactionEventValidator{t, payload}
}
