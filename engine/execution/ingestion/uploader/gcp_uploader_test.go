package uploader

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	testutils "github.com/onflow/flow-go/utils/unittest"
)

func Test_GCPBucketUploader(t *testing.T) {
	testutils.SkipUnless(t, testutils.TEST_REQUIRES_GCP_ACCESS, "requires GCP Bucket setup")

	bucketName := os.Getenv("FLOW_TEST_GCP_BUCKET_NAME")
	if bucketName == "" {
		t.Fatal("please set FLOW_TEST_GCP_BUCKET_NAME environmental variable")
	}
	uploader, err := NewGCPBucketUploader(context.Background(), bucketName, zerolog.Nop())
	require.NoError(t, err)

	cr, _ := generateComputationResult(t)

	buffer := &bytes.Buffer{}
	err = WriteComputationResultsTo(cr, buffer)
	require.NoError(t, err)

	err = uploader.Upload(cr)

	require.NoError(t, err)

	// check uploaded object
	client, err := storage.NewClient(context.Background())
	require.NoError(t, err)
	bucket := client.Bucket(bucketName)

	objectName := GCPBlockDataObjectName(cr)

	reader, err := bucket.Object(objectName).NewReader(context.Background())
	require.NoError(t, err)

	readBytes, err := io.ReadAll(reader)
	require.NoError(t, err)

	require.Equal(t, buffer.Bytes(), readBytes)
}
