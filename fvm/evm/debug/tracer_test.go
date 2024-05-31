package debug

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type mockUploader struct {
	uploadFunc func(string, json.RawMessage) error
}

func (m mockUploader) Upload(id string, data json.RawMessage) error {
	return m.uploadFunc(id, data)
}

var _ Uploader = &mockUploader{}

func TestCallTracer(t *testing.T) {
	t.Run("collect traces and upload them", func(t *testing.T) {
		mockUpload := &mockUploader{
			uploadFunc: func(id string, message json.RawMessage) error {
				fmt.Println(id, string(message))
				return nil
			},
		}

		tracer, err := NewEVMCallTracer(mockUpload)
		require.NoError(t, err)

		from := gethCommon.HexToAddress("0x01")
		to := gethCommon.HexToAddress("0x02")

		tracer.TxTracer().CaptureStart(
			nil,
			from,
			to,
			true,
			[]byte{0x01, 0x02},
			10,
			big.NewInt(1),
		)
		tracer.TxTracer().CaptureTxStart(100)
		tracer.TxTracer().CaptureEnter(vm.ADD, from, to, []byte{0x02, 0x04}, 20, big.NewInt(2))
		tracer.TxTracer().CaptureTxEnd(500)
		tracer.TxTracer().CaptureEnd([]byte{0x02}, 200, nil)

		tracer.Collect(gethCommon.Hash{0x05})
	})
}
