package state_stream

import (
	"bytes"
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	headers *storagemock.Headers
	seals   *storagemock.Seals
	results *storagemock.ExecutionResults
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	suite.headers = storagemock.NewHeaders(suite.T())
	suite.seals = storagemock.NewSeals(suite.T())
	suite.results = storagemock.NewExecutionResults(suite.T())
}

func (suite *Suite) TestGetExecutionDataByBlockID() {

	// create the handler with the mock
	bs := blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	eds := execution_data.NewExecutionDataStore(bs, execution_data.DefaultSerializer)
	client, err := New(suite.headers, suite.seals, suite.results, eds, unittest.Logger())
	require.NoError(suite.T(), err)

	// mock parameters
	ctx := context.Background()
	blockHeader := unittest.BlockHeaderFixture()

	seals := unittest.BlockSealsFixture(1)[0]
	results := unittest.ExecutionResultFixture()
	numChunks := 5
	minSerializedSize := 5 * execution_data.DefaultMaxBlobSize
	chunks := make([]*execution_data.ChunkExecutionData, numChunks)

	for i := 0; i < numChunks; i++ {
		chunks[i] = generateChunkExecutionData(suite.T(), uint64(minSerializedSize))
	}

	execData := &execution_data.BlockExecutionData{
		BlockID:             blockHeader.ID(),
		ChunkExecutionDatas: chunks,
	}

	execDataRes, err := convert.BlockExecutionDataToMessage(execData)
	require.Nil(suite.T(), err)

	suite.headers.On("ByBlockID", blockHeader.ID()).Return(blockHeader, nil)
	suite.seals.On("FinalizedSealForBlock", blockHeader.ID()).Return(seals, nil)
	suite.results.On("ByID", seals.ResultID).Return(results, nil)
	suite.Run("happy path TestGetExecutionDataByBlockID success", func() {
		resID, err := eds.AddExecutionData(ctx, execData)
		assert.NoError(suite.T(), err)
		results.ExecutionDataID = resID
		res, err := client.GetExecutionDataByBlockID(ctx, blockHeader.ID())
		assert.Equal(suite.T(), execDataRes, res)
		assert.NoError(suite.T(), err)
	})

	suite.Run("missing exec data for TestGetExecutionDataByBlockID failure", func() {
		results.ExecutionDataID = unittest.IdentifierFixture()
		execDataRes, err := client.GetExecutionDataByBlockID(ctx, blockHeader.ID())
		assert.Nil(suite.T(), execDataRes)
		var blobNotFoundError *execution_data.BlobNotFoundError
		assert.ErrorAs(suite.T(), err, &blobNotFoundError)
	})

	suite.headers.AssertExpectations(suite.T())
	suite.seals.AssertExpectations(suite.T())
	suite.results.AssertExpectations(suite.T())
}

func generateChunkExecutionData(t *testing.T, minSerializedSize uint64) *execution_data.ChunkExecutionData {
	ced := &execution_data.ChunkExecutionData{
		TrieUpdate: testutils.TrieUpdateFixture(1, 1, 8),
	}

	size := 1

	for {
		buf := &bytes.Buffer{}
		require.NoError(t, execution_data.DefaultSerializer.Serialize(buf, ced))
		if buf.Len() >= int(minSerializedSize) {
			return ced
		}

		v := make([]byte, size)
		_, _ = rand.Read(v)

		k, err := ced.TrieUpdate.Payloads[0].Key()
		require.NoError(t, err)

		ced.TrieUpdate.Payloads[0] = ledger.NewPayload(k, v)
		size *= 2
	}
}
