package types_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/onflow/cadence/encoding/ccf"
	cdcCommon "github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestEVMEventsCCFEncodingDecoding(t *testing.T) {

	t.Run("evm.BlockExecuted", func(t *testing.T) {
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

		v, err := ccf.Encode(ev)
		require.NoError(t, err)
		assert.Equal(t, ccf.HasMsgPrefix(v), true)

		evt, err := ccf.Decode(nil, v)
		require.NoError(t, err)

		assert.Equal(t, evt.Type().ID(), "evm.BlockExecuted")

		location, qualifiedIdentifier, err := cdcCommon.DecodeTypeID(nil, "evm.BlockExecuted")
		require.NoError(t, err)

		assert.Equal(t, types.EVMLocation{}, location)
		assert.Equal(t, "BlockExecuted", qualifiedIdentifier)
	})

	t.Run("evm.TransactionExecuted", func(t *testing.T) {
		txEncoded, err := hex.DecodeString("fff83b81ff0194000000000000000000000000000000000000000094000000000000000000000000000000000000000180895150ae84a8cdf00000825208")
		require.NoError(t, err)
		txHash := gethCommon.HexToHash("0xf3593c3eb9ac1125f31acfa5520877f223279f6238a670936e65a710ccd44641")
		data, err := hex.DecodeString("000000000000000000000000000000000000000000000000000000000000002a")
		require.NoError(t, err)
		log := &gethTypes.Log{
			Index:       0,
			BlockNumber: 2,
			BlockHash:   gethCommon.HexToHash("0xf3593c3eb9ac1125f31acfa5520877f223279f6238a670936e65a710ccd44641"),
			TxHash:      txHash,
			TxIndex:     0,
			Address:     gethCommon.HexToAddress("0x99466ed2e37b892a2ee3e9cd55a98b68f5735db2"),
			Data:        data,
			Topics: []gethCommon.Hash{
				gethCommon.HexToHash("0x24abdb5865df5079dcc5ac590ff6f01d5c16edbc5fab4e195d9febd1114503da"),
			},
		}
		txResult := &types.Result{
			Failed:                  false,
			TxType:                  255,
			GasConsumed:             23200,
			DeployedContractAddress: types.EmptyAddress,
			ReturnedValue:           data,
			Logs:                    []*gethTypes.Log{log},
		}

		event := types.NewTransactionExecutedEvent(
			2,
			txEncoded,
			txHash,
			txResult,
		)
		ev, err := event.Payload.CadenceEvent()
		require.NoError(t, err)

		v, err := ccf.Encode(ev)
		require.NoError(t, err)
		assert.Equal(t, ccf.HasMsgPrefix(v), true)

		evt, err := ccf.Decode(nil, v)
		require.NoError(t, err)

		assert.Equal(t, evt.Type().ID(), "evm.TransactionExecuted")

		location, qualifiedIdentifier, err := cdcCommon.DecodeTypeID(nil, "evm.TransactionExecuted")
		require.NoError(t, err)

		assert.Equal(t, types.EVMLocation{}, location)
		assert.Equal(t, "TransactionExecuted", qualifiedIdentifier)
	})
}
