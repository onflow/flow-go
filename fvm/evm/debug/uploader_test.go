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
	"github.com/onflow/go-ethereum/core/vm"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	testutils "github.com/onflow/flow-go/utils/unittest"
)

func Test_Uploader(t *testing.T) {
	testutils.SkipUnless(t, testutils.TEST_REQUIRES_GCP_ACCESS, "requires GCP Bucket setup")

	t.Run("successfuly upload traces", func(t *testing.T) {
		uploader, err := NewGCPUploader()
		require.NoError(t, err)

		const testID = "test_1"

		data := json.RawMessage(fmt.Sprintf(`{ "test": %d }`, rand.Int()))

		err = uploader.Upload(testID, data)
		require.NoError(t, err)

		// check uploaded object
		client, err := storage.NewClient(context.Background())
		require.NoError(t, err)
		bucket := client.Bucket(bucketName)

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
		uploader, err := NewGCPUploader()
		require.NoError(t, err)

		tracer, err := NewEVMCallTracer(uploader, zerolog.Nop())
		require.NoError(t, err)

		tr := tracer.TxTracer()
		require.NotNil(t, tr)

		tr.CaptureTxStart(1000)
		tr.CaptureEnter(vm.ADD, gethCommon.HexToAddress("0x01"), gethCommon.HexToAddress("0x02"), []byte{0x01}, 10, big.NewInt(2))
		tr.CaptureTxEnd(500)

		traces, err := tr.GetResult()
		require.NoError(t, err)

		id := gethCommon.BytesToHash([]byte("test-tx"))
		tracer.Collect(id)

		// check uploaded object
		client, err := storage.NewClient(context.Background())
		require.NoError(t, err)
		bucket := client.Bucket(bucketName)

		reader, err := bucket.
			Object(id.String()).
			NewReader(context.Background())
		require.NoError(t, err)

		readBytes, err := io.ReadAll(reader)
		require.NoError(t, err)

		require.Equal(t, []byte(traces), readBytes)
	})
}
