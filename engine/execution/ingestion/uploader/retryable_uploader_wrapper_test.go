package uploader

import (
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	executionDataMock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"

	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/mock"

	storageMock "github.com/onflow/flow-go/storage/mock"

	"gotest.tools/assert"

	"github.com/onflow/flow-go/engine/execution"
)

func Test_Upload_invoke(t *testing.T) {
	wg := sync.WaitGroup{}
	uploaderCalled := false

	dummyUploader := &DummyUploader{
		f: func() error {
			uploaderCalled = true
			wg.Done()
			return nil
		},
	}
	asyncUploader := NewAsyncUploader(dummyUploader,
		1*time.Nanosecond, 1, zerolog.Nop(), &metrics.NoopCollector{})

	testRetryableUploaderWrapper := createTestBadgerRetryableUploaderWrapper(asyncUploader)
	defer testRetryableUploaderWrapper.Done()

	// nil input - no call to Upload()
	err := testRetryableUploaderWrapper.Upload(nil)
	assert.Assert(t, err != nil)
	assert.Assert(t, !uploaderCalled)

	// non-nil input - Upload() should be called
	wg.Add(1)
	testComputationResult := createTestComputationResult()
	err = testRetryableUploaderWrapper.Upload(testComputationResult)
	wg.Wait()

	assert.NilError(t, err)
	assert.Assert(t, uploaderCalled)
}

func Test_RetryUpload(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	uploaderCalled := false
	dummyUploader := &DummyUploader{
		f: func() error {
			uploaderCalled = true
			wg.Done()
			return nil
		},
	}
	asyncUploader := NewAsyncUploader(dummyUploader,
		1*time.Nanosecond, 1, zerolog.Nop(), &metrics.NoopCollector{})

	testRetryableUploaderWrapper := createTestBadgerRetryableUploaderWrapper(asyncUploader)
	defer testRetryableUploaderWrapper.Done()

	err := testRetryableUploaderWrapper.RetryUpload()
	wg.Wait()

	assert.NilError(t, err)
	assert.Assert(t, uploaderCalled)
}

func Test_AsyncUploaderCallback(t *testing.T) {
	wgUploadCalleded := sync.WaitGroup{}
	wgUploadCalleded.Add(1)

	uploader := &DummyUploader{
		f: func() error {
			wgUploadCalleded.Done()
			return nil
		},
	}
	asyncUploader := NewAsyncUploader(uploader,
		1*time.Nanosecond, 1, zerolog.Nop(), &metrics.NoopCollector{})

	testRetryableUploaderWrapper := createTestBadgerRetryableUploaderWrapper(asyncUploader)
	defer testRetryableUploaderWrapper.Done()

	testComputationResult := createTestComputationResult()
	err := testRetryableUploaderWrapper.Upload(testComputationResult)
	assert.NilError(t, err)

	wgUploadCalleded.Wait()
}

func Test_ReconstructComputationResultFromStorage(t *testing.T) {
	// test data
	testBlockID := flow.HashToID([]byte{1, 2, 3})
	testEDID := flow.HashToID([]byte{4, 5, 6})
	testTrieUpdateRootHash, _ := ledger.ToRootHash([]byte{7, 8, 9})
	testTrieUpdate := &ledger.TrieUpdate{
		RootHash: testTrieUpdateRootHash,
	}
	testChunkExecutionDatas := []*execution_data.ChunkExecutionData{
		{
			TrieUpdate: testTrieUpdate,
		},
	}
	testEvents := []flow.Event{
		unittest.EventFixture(flow.EventAccountCreated, 0, 0, flow.HashToID([]byte{11, 22, 33}), 200),
	}
	testCollectionID := flow.HashToID([]byte{0xA, 0xB, 0xC})
	testBlock := &flow.Block{
		Header: &flow.Header{},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{
				{
					CollectionID: testCollectionID,
				},
			},
		},
	}
	testTransactionBody := &flow.TransactionBody{
		Script: []byte("random script"),
	}
	testCollectionData := &flow.Collection{
		Transactions: []*flow.TransactionBody{testTransactionBody},
	}
	testTransactionResult := flow.TransactionResult{ /*TODO*/ }
	testStateCommit := flow.StateCommitment{}

	// mock storage interfaces

	mockBlocksStorage := new(storageMock.Blocks)
	mockBlocksStorage.On("ByID", testBlockID).Return(testBlock, nil)

	mockCommitsStorage := new(storageMock.Commits)
	mockCommitsStorage.On("ByBlockID", testBlockID).Return(testStateCommit, nil)

	mockCollectionsStorage := new(storageMock.Collections)
	mockCollectionsStorage.On("ByID", testCollectionID).Return(testCollectionData, nil)

	mockEventsStorage := new(storageMock.Events)
	mockEventsStorage.On("ByBlockID", testBlockID).Return(testEvents, nil)

	mockResultsStorage := new(storageMock.ExecutionResults)
	mockResultsStorage.On("ByBlockID", testBlockID).Return(&flow.ExecutionResult{
		ExecutionDataID: testEDID,
	}, nil)

	mockTransactionResultsStorage := new(storageMock.TransactionResults)
	mockTransactionResultsStorage.On("ByBlockID", testBlockID).Return([]flow.TransactionResult{testTransactionResult}, nil)

	mockComputationResultStorage := new(storageMock.ComputationResultUploadStatus)
	mockComputationResultStorage.On("GetIDsByUploadStatus", mock.Anything).Return([]flow.Identifier{testBlockID}, nil)
	testComputationResultUploadStatus := false
	mockComputationResultStorage.On("ByID", testBlockID).Return(testComputationResultUploadStatus, nil)
	mockComputationResultStorage.On("Upsert", testBlockID, mock.Anything).Return(nil)

	mockExecutionDataDowloader := new(executionDataMock.Downloader)
	mockExecutionDataDowloader.On("Get", mock.Anything, testEDID).Return(
		&execution_data.BlockExecutionData{
			BlockID:             testBlockID,
			ChunkExecutionDatas: testChunkExecutionDatas,
		}, nil)

	dummyUploader := &DummyUploader{
		f: func() error {
			return nil
		},
	}
	testRetryableUploaderWrapper := NewBadgerRetryableUploaderWrapper(
		NewAsyncUploader(dummyUploader,
			1*time.Nanosecond, 1, zerolog.Nop(), &metrics.NoopCollector{}),
		mockBlocksStorage,
		mockCommitsStorage,
		mockCollectionsStorage,
		mockEventsStorage,
		mockResultsStorage,
		mockTransactionResultsStorage,
		mockComputationResultStorage,
		mockExecutionDataDowloader,
		&metrics.NoopCollector{},
	)

	reconstructedComputationResult, err := testRetryableUploaderWrapper.reconstructComputationResult(testBlockID)
	assert.NilError(t, err)

	expectedCompleteCollections := make([]*entity.CompleteCollection, 1)
	expectedCompleteCollections[0] = &entity.CompleteCollection{
		Guarantee: &flow.CollectionGuarantee{
			CollectionID: testCollectionID,
		},
		Transactions: []*flow.TransactionBody{testTransactionBody},
	}

	expectedTestEvents := make([]*flow.Event, len(testEvents))
	for i, event := range testEvents {
		expectedTestEvents[i] = &event
	}

	expectedBlockData := &BlockData{
		Block:                testBlock,
		Collections:          expectedCompleteCollections,
		TxResults:            []*flow.TransactionResult{&testTransactionResult},
		Events:               expectedTestEvents,
		TrieUpdates:          []*ledger.TrieUpdate{testTrieUpdate},
		FinalStateCommitment: testStateCommit,
	}

	assert.DeepEqual(
		t,
		expectedBlockData,
		ComputationResultToBlockData(reconstructedComputationResult),
	)
}

// createTestBadgerRetryableUploaderWrapper() create BadgerRetryableUploaderWrapper instance with given
// AsyncUploader instance and proper mock storage and EDS interfaces.
func createTestBadgerRetryableUploaderWrapper(asyncUploader *AsyncUploader) *BadgerRetryableUploaderWrapper {
	mockBlocksStorage := new(storageMock.Blocks)
	mockBlocksStorage.On("ByID", mock.Anything).Return(&flow.Block{
		Header:  &flow.Header{},
		Payload: nil,
	}, nil)

	mockCommitsStorage := new(storageMock.Commits)
	mockCommitsStorage.On("ByBlockID", mock.Anything).Return(nil, nil)

	mockCollectionsStorage := new(storageMock.Collections)

	mockEventsStorage := new(storageMock.Events)
	mockEventsStorage.On("ByBlockID", mock.Anything).Return([]flow.Event{}, nil)

	mockResultsStorage := new(storageMock.ExecutionResults)
	mockResultsStorage.On("ByBlockID", mock.Anything).Return(&flow.ExecutionResult{
		ExecutionDataID: flow.ZeroID,
	}, nil)

	mockTransactionResultsStorage := new(storageMock.TransactionResults)
	mockTransactionResultsStorage.On("ByBlockID", mock.Anything).Return(nil, nil)

	mockComputationResultStorage := new(storageMock.ComputationResultUploadStatus)
	testID := flow.HashToID([]byte{1, 2, 3})
	mockComputationResultStorage.On("GetIDsByUploadStatus", mock.Anything).Return([]flow.Identifier{testID}, nil)
	testComputationResultUploadStatus := false
	mockComputationResultStorage.On("ByID", testID).Return(testComputationResultUploadStatus, nil)
	mockComputationResultStorage.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	mockExecutionDataDowloader := new(executionDataMock.Downloader)
	mockExecutionDataDowloader.On("Get", mock.Anything, mock.Anything).Return(
		&execution_data.BlockExecutionData{
			BlockID:             flow.ZeroID,
			ChunkExecutionDatas: make([]*execution_data.ChunkExecutionData, 0),
		}, nil)

	return NewBadgerRetryableUploaderWrapper(
		asyncUploader,
		mockBlocksStorage,
		mockCommitsStorage,
		mockCollectionsStorage,
		mockEventsStorage,
		mockResultsStorage,
		mockTransactionResultsStorage,
		mockComputationResultStorage,
		mockExecutionDataDowloader,
		&metrics.NoopCollector{},
	)
}

// createTestComputationResult() creates ComputationResult with valid ExecutableBlock ID
func createTestComputationResult() *execution.ComputationResult {
	blockA := unittest.BlockHeaderFixture()
	start := unittest.StateCommitmentFixture()
	blockB := unittest.ExecutableBlockFixtureWithParent(nil, blockA, &start)
	testComputationResult := execution.NewEmptyComputationResult(blockB)
	return testComputationResult
}
