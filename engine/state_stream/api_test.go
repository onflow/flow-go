package state_stream

import (
	"bytes"
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	downloadermock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	blocks     *storagemock.Blocks
	headers    *storagemock.Headers
	seals      *storagemock.Seals
	results    *storagemock.ExecutionResults
	downloader *downloadermock.Downloader
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	header := unittest.BlockHeaderFixture()
	params := new(protocol.Params)
	params.On("Root").Return(header, nil)
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.seals = new(storagemock.Seals)
	suite.results = new(storagemock.ExecutionResults)

	suite.downloader = new(downloadermock.Downloader)
}

func (suite *Suite) TestGetExecutionDataByBlockID() {

	// create the handler with the mock
	client := New(suite.headers, suite.seals, suite.results, suite.downloader)

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
		suite.downloader.On("Download", ctx, results.ExecutionDataID).Return(execData, nil).Once()
		res, err := client.GetExecutionDataByBlockID(ctx, blockHeader.ID())
		suite.downloader.AssertExpectations(suite.T())
		assert.Equal(suite.T(), execDataRes, res)
		assert.NoError(suite.T(), err)
	})

	suite.Run("missing exec data for TestGetExecutionDataByBlockID failure", func() {
		suite.downloader.On("Download", ctx, results.ExecutionDataID).Return(nil, storage.ErrNotFound).Once()
		_, err := client.GetExecutionDataByBlockID(ctx, blockHeader.ID())
		suite.Require().Error(err)
	})
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
