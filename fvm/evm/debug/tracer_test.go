package debug

import (
	"encoding/json"
	"math/big"
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	"github.com/onflow/go-ethereum/core/vm"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func Test_CallTracer(t *testing.T) {
	t.Run("collect traces and upload them", func(t *testing.T) {
		txID := gethCommon.Hash{0x05}
		blockID := flow.Identifier{0x01}
		var res json.RawMessage

		mockUpload := &testutils.MockUploader{
			UploadFunc: func(id string, message json.RawMessage) error {
				require.Equal(t, TraceID(txID, blockID), id)
				require.Equal(t, res, message)
				return nil
			},
		}

		tracer, err := NewEVMCallTracer(mockUpload, zerolog.Nop())
		require.NoError(t, err)
		tracer.WithBlockID(blockID)

		from := gethCommon.HexToAddress("0x01")
		to := gethCommon.HexToAddress("0x02")
		nonce := uint64(10)
		data := []byte{0x02, 0x04}
		amount := big.NewInt(1)

		// first transaction
		tr := tracer.TxTracer()
		require.NotNil(t, tr)
		tx := gethTypes.NewTransaction(nonce, to, amount, 100, big.NewInt(10), data)
		tr.OnTxStart(nil, tx, from)
		tr.OnEnter(0, 0, from, to, []byte{0x01, 0x02}, 10, big.NewInt(1))
		tr.OnEnter(1, byte(vm.ADD), from, to, data, 20, big.NewInt(2))
		tr.OnExit(1, nil, 10, nil, false)
		tr.OnExit(0, []byte{0x02}, 200, nil, false)
		tr.OnTxEnd(&gethTypes.Receipt{TxHash: tx.Hash()}, nil)
		tracer.Collect(tx.Hash())
	})

	t.Run("collect traces and upload them (after a failed transaction)", func(t *testing.T) {
		txID := gethCommon.Hash{0x05}
		blockID := flow.Identifier{0x01}
		var res json.RawMessage

		mockUpload := &testutils.MockUploader{
			UploadFunc: func(id string, message json.RawMessage) error {
				require.Equal(t, TraceID(txID, blockID), id)
				require.Equal(t, res, message)
				return nil
			},
		}

		tracer, err := NewEVMCallTracer(mockUpload, zerolog.Nop())
		require.NoError(t, err)
		tracer.WithBlockID(blockID)

		from := gethCommon.HexToAddress("0x01")
		to := gethCommon.HexToAddress("0x02")
		nonce := uint64(10)
		data := []byte{0x02, 0x04}
		amount := big.NewInt(1)

		// first transaction
		tr := tracer.TxTracer()
		require.NotNil(t, tr)

		// failed transaction (no exit and collect call)
		tx := gethTypes.NewTransaction(nonce, to, amount, 300, big.NewInt(30), data)
		tr.OnTxStart(nil, tx, from)
		tr.OnEnter(0, 0, from, to, []byte{0x01, 0x02}, 10, big.NewInt(1))
		tr.OnEnter(1, byte(vm.ADD), from, to, data, 20, big.NewInt(2))

		// 4th transaction
		tx = gethTypes.NewTransaction(nonce, to, amount, 400, big.NewInt(40), data)
		tr.OnTxStart(nil, tx, from)
		tr.OnEnter(0, 0, from, to, []byte{0x01, 0x02}, 10, big.NewInt(1))
		tr.OnExit(0, []byte{0x02}, 200, nil, false)
		tr.OnTxEnd(&gethTypes.Receipt{TxHash: tx.Hash()}, nil)
		tracer.Collect(tx.Hash())
	})

	t.Run("collector panic recovery", func(t *testing.T) {
		txID := gethCommon.Hash{0x05}
		blockID := flow.Identifier{0x01}

		mockUpload := &testutils.MockUploader{
			UploadFunc: func(id string, message json.RawMessage) error {
				panic("nooooo")
			},
		}

		tracer, err := NewEVMCallTracer(mockUpload, zerolog.Nop())
		require.NoError(t, err)
		tracer.WithBlockID(blockID)

		require.NotPanics(t, func() {
			tracer.Collect(txID)
		})
	})

	t.Run("nop tracer", func(t *testing.T) {
		tracer := nopTracer{}
		require.Nil(t, tracer.TxTracer())
	})
}
