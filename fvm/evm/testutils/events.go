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

func (tx *TransactionEventValidator) HasIndex(index uint16) *TransactionEventValidator {
	require.Equal(tx.t, index, int(tx.payload.Index))
	return tx
}

func (tx *TransactionEventValidator) HasBlockHash(blockHash string) *TransactionEventValidator {
	require.Equal(tx.t, blockHash, tx.payload.BlockHash)
	return tx
}

func (tx *TransactionEventValidator) HasTxType(txType uint8) *TransactionEventValidator {
	require.Equal(tx.t, txType, tx.payload.TransactionType)
	return tx
}

func (tx *TransactionEventValidator) HasTxHash(txHash string) *TransactionEventValidator {
	require.Equal(tx.t, txHash, tx.payload.Hash)
	return tx
}

func (tx *TransactionEventValidator) HasErrorCode(code types.ErrorCode) *TransactionEventValidator {
	require.Equal(tx.t, code, int(tx.payload.ErrorCode))
	return tx
}

func (tx *TransactionEventValidator) HasErrorMessage(msg string) *TransactionEventValidator {
	require.Equal(tx.t, msg, tx.payload.ErrorMessage)
	return tx
}

func (tx *TransactionEventValidator) HasGasConsumed(gas uint64) *TransactionEventValidator {
	require.Equal(tx.t, gas, tx.payload.GasConsumed)
	return tx
}

func (tx *TransactionEventValidator) HasReturnedData(retData []byte) *TransactionEventValidator {
	require.Equal(tx.t, hex.EncodeToString(retData), tx.payload.ReturnedData)
	return tx
}

func (tx *TransactionEventValidator) HasDeployedContractAddress(addr *types.Address) *TransactionEventValidator {
	deployedAddress := ""
	if addr != nil {
		deployedAddress = addr.String()
	}
	require.Equal(tx.t, deployedAddress, tx.payload.ContractAddress)
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

func (tx *TransactionEventValidator) MatchesResult(result *types.Result) *TransactionEventValidator {
	tx.HasTxHash(result.TxHash.String())
	tx.HasIndex(result.Index)
	tx.HasTxType(result.TxType)
	tx.HasErrorCode(result.ResultSummary().ErrorCode)
	tx.HasErrorMessage(result.ResultSummary().ErrorMessage)
	tx.HasGasConsumed(result.GasConsumed)
	tx.HasDeployedContractAddress(result.DeployedContractAddress)
	tx.HasReturnedData(result.ReturnedData)
	tx.HasLogs(result.Logs)
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
	// 	location := common.NewAddressLocation(nil, common.Address(h.evmContractAddress), "EVM")
	// require.Equal(t, cev.EventType, types.EventTypeTransactionExecuted)
	payload, err := types.DecodeTransactionEventPayload(cev)
	require.NoError(t, err)
	return &TransactionEventValidator{t, payload}
}
