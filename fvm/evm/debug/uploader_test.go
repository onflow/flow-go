package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
)

func Test_Uploader(t *testing.T) {
	testutils.SkipUnless(t, testutils.TEST_REQUIRES_GCP_ACCESS, "requires GCP Bucket setup")

	t.Run("successfuly upload traces", func(t *testing.T) {
		err := os.Setenv("STORAGE_EMULATOR_HOST", "localhost:9023")

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
