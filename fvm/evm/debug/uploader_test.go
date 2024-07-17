package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"testing"

	"cloud.google.com/go/storage"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	"github.com/onflow/go-ethereum/core/vm"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	testutils "github.com/onflow/flow-go/utils/unittest"
)

// we use previewnet bucket for integraiton test
const bucket = "previewnet1-evm-execution-traces"

func Test_Uploader(t *testing.T) {
	testutils.SkipUnless(t, testutils.TEST_REQUIRES_GCP_ACCESS, "requires GCP Bucket setup")

	t.Run("successfuly upload traces", func(t *testing.T) {
		uploader, err := NewGCPUploader(bucket)
		require.NoError(t, err)

		const testID = "test_p"

		data := json.RawMessage(fmt.Sprintf(`{ "test": %d }`, rand.Int()))

		err = uploader.Upload(testID, data)
		require.NoError(t, err)

		// check uploaded object
		client, err := storage.NewClient(context.Background())
		require.NoError(t, err)
		bucket := client.Bucket(bucket)

		reader, err := bucket.Object(testID).NewReader(context.Background())
		require.NoError(t, err)

		readBytes, err := io.ReadAll(reader)
		require.NoError(t, err)

		require.Equal(t, []byte(data), readBytes)
	})
}

func Test_TracerUploaderIntegration(t *testing.T) {
	testutils.SkipUnless(t, testutils.TEST_REQUIRES_GCP_ACCESS, "requires GCP Bucket setup")

	t.Run("successfuly uploads traces", func(t *testing.T) {
		uploader, err := NewGCPUploader(bucket)
		require.NoError(t, err)

		tracer, err := NewEVMCallTracer(uploader, zerolog.Nop())
		require.NoError(t, err)

		tr := tracer.TxTracer()
		require.NotNil(t, tr)

		from := gethCommon.HexToAddress("0x01")
		to := gethCommon.HexToAddress("0x02")
		nonce := uint64(10)
		gas := uint64(100)
		gasPrice := big.NewInt(1)
		value := big.NewInt(2)
		input := []byte{0x01}
		tx := gethTypes.NewTransaction(nonce, to, value, gas, gasPrice, input)

		tr.OnTxStart(nil, tx, from)
		tr.OnEnter(0, byte(vm.ADD), from, to, input, gas, value)
		tr.OnTxEnd(&gethTypes.Receipt{}, nil)

		traces, err := tr.GetResult()
		require.NoError(t, err)

		id := gethCommon.BytesToHash([]byte("test-tx"))
		blockID := flow.Identifier{0x02}
		tracer.WithBlockID(blockID)
		tracer.Collect(id)

		// check uploaded object
		client, err := storage.NewClient(context.Background())
		require.NoError(t, err)
		bucket := client.Bucket(bucket)

		reader, err := bucket.
			Object(id.String()).
			NewReader(context.Background())
		require.NoError(t, err)

		readBytes, err := io.ReadAll(reader)
		require.NoError(t, err)

		require.Equal(t, []byte(traces), readBytes)
	})
}
