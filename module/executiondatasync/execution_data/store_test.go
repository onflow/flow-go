package execution_data_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goassert "gotest.tools/assert"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/utils/unittest"
)

var defaultSerializer = execution_data.NewSerializer(new(cbor.Codec), compressor.NewLz4Compressor())

func getBlobstore() blobs.Blobstore {
	return blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
}

func getExecutionDataStore(blobstore blobs.Blobstore, serializer execution_data.Serializer) execution_data.ExecutionDataStore {
	return execution_data.NewExecutionDataStore(blobstore, serializer)
}

func generateChunkExecutionData(t *testing.T, minSerializedSize uint64) *execution_data.ChunkExecutionData {
	ced := &execution_data.ChunkExecutionData{
		TrieUpdate: &ledger.TrieUpdate{
			Payloads: []*ledger.Payload{
				{
					Value: nil,
				},
			},
		},
	}

	size := 1

	for {
		buf := &bytes.Buffer{}
		require.NoError(t, defaultSerializer.Serialize(buf, ced))

		if buf.Len() >= int(minSerializedSize) {
			t.Logf("Chunk execution data size: %d", buf.Len())
			return ced
		}

		v := make([]byte, size)
		rand.Read(v)
		ced.TrieUpdate.Payloads[0].Value = v
		size *= 2
	}
}

func generateBlockExecutionData(t *testing.T, numChunks int, minSerializedSizePerChunk uint64) *execution_data.BlockExecutionData {
	bed := &execution_data.BlockExecutionData{
		BlockID:             unittest.IdentifierFixture(),
		ChunkExecutionDatas: make([]*execution_data.ChunkExecutionData, numChunks),
	}

	for i := 0; i < numChunks; i++ {
		bed.ChunkExecutionDatas[i] = generateChunkExecutionData(t, minSerializedSizePerChunk)
	}

	return bed
}

func getExecutionData(
	eds execution_data.ExecutionDataStore,
	rootID flow.Identifier,
	timeout time.Duration,
) (*execution_data.BlockExecutionData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return eds.GetExecutionData(ctx, rootID)
}

func addExecutionData(
	eds execution_data.ExecutionDataStore,
	bed *execution_data.BlockExecutionData,
	timeout time.Duration,
) (flow.Identifier, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return eds.AddExecutionData(ctx, bed)
}

func TestHappyPath(t *testing.T) {
	t.Parallel()

	eds := getExecutionDataStore(getBlobstore(), defaultSerializer)

	test := func(numChunks int, minSerializedSizePerChunk uint64) {
		expected := generateBlockExecutionData(t, numChunks, minSerializedSizePerChunk)
		rootId, err := addExecutionData(eds, expected, time.Second)
		require.NoError(t, err)
		actual, err := getExecutionData(eds, rootId, time.Second)
		require.NoError(t, err)
		goassert.DeepEqual(t, expected, actual)
	}

	test(1, 0)                                   // small execution data (single level blob tree)
	test(1, 5*execution_data.DefaultMaxBlobSize) // large execution data (multi level blob tree)
}

type randomSerializer struct{}

func (rs *randomSerializer) Serialize(w io.Writer, v interface{}) error {
	data := make([]byte, 1024)
	rand.Read(data)
	_, err := w.Write(data)
	return err
}

func (rs *randomSerializer) Deserialize(r io.Reader) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

type corruptedTailSerializer struct {
	corruptedChunk int
	i              int
}

func newCorruptedTailSerializer(numChunks int) *corruptedTailSerializer {
	return &corruptedTailSerializer{
		corruptedChunk: rand.Intn(numChunks) + 1,
	}
}

func (cts *corruptedTailSerializer) Serialize(w io.Writer, v interface{}) error {
	if _, ok := v.(*execution_data.ChunkExecutionData); ok {
		cts.i++
		if cts.i == cts.corruptedChunk {
			buf := &bytes.Buffer{}

			err := defaultSerializer.Serialize(buf, v)
			if err != nil {
				return err
			}

			data := buf.Bytes()
			rand.Read(data[len(data)-1024:])

			_, err = w.Write(data)
			return err
		}
	}

	return defaultSerializer.Serialize(w, v)
}

func (cts *corruptedTailSerializer) Deserialize(r io.Reader) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestMalformedData(t *testing.T) {
	t.Parallel()

	test := func(bed *execution_data.BlockExecutionData, serializer execution_data.Serializer) {
		blobstore := getBlobstore()
		defaultEds := getExecutionDataStore(blobstore, defaultSerializer)
		malformedEds := getExecutionDataStore(blobstore, serializer)
		rootID, err := addExecutionData(malformedEds, bed, time.Second)
		require.NoError(t, err)
		_, err = getExecutionData(defaultEds, rootID, time.Second)
		var malformedDataErr *execution_data.MalformedDataError
		assert.ErrorAs(t, err, &malformedDataErr)
	}

	numChunks := 5
	bed := generateBlockExecutionData(t, numChunks, 10*execution_data.DefaultMaxBlobSize)

	test(bed, &randomSerializer{})                   // random bytes
	test(bed, newCorruptedTailSerializer(numChunks)) // serialized execution data with random bytes replaced at the end of a random chunk
}
