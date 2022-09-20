package uploader

import (
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
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
			wg.Done()
			uploaderCalled = true
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
			wg.Done()
			uploaderCalled = true
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
	testChunkExecutionDatas := []*execution_data.ChunkExecutionData{
		{
			TrieUpdate: &ledger.TrieUpdate{
				RootHash: testTrieUpdateRootHash,
			},
		},
	}
	testEvents := []flow.Event{
		unittest.EventFixture(flow.EventAccountCreated, 1, 0, flow.HashToID([]byte{11, 22, 33}), 200),
	}
	testCollectionID := flow.HashToID([]byte{0xA, 0xB, 0xC})
	testBlock := &flow.Block{
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
	mockExecutionDataDowloader.On("Download", mock.Anything, testEDID).Return(
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

	expectedCompleteCollections := make(map[flow.Identifier]*entity.CompleteCollection)
	expectedCompleteCollections[testCollectionID] = &entity.CompleteCollection{
		Guarantee: &flow.CollectionGuarantee{
			CollectionID: testCollectionID,
		},
		Transactions: []*flow.TransactionBody{testTransactionBody},
	}
	expectedComputationResult := &execution.ComputationResult{
		ExecutableBlock: &entity.ExecutableBlock{
			Block:               testBlock,
			CompleteCollections: expectedCompleteCollections,
		},
		Events: []flow.EventsList{testEvents},
		TransactionResults: []flow.TransactionResult{
			testTransactionResult,
		},
		StateCommitments: []flow.StateCommitment{testStateCommit},
		TrieUpdates: []*ledger.TrieUpdate{
			{
				RootHash: testTrieUpdateRootHash,
			},
		},
	}

	assert.DeepEqual(t, reconstructedComputationResult, expectedComputationResult,
		cmpopts.IgnoreUnexported(entity.ExecutableBlock{}))
}

// createTestBadgerRetryableUploaderWrapper() create BadgerRetryableUploaderWrapper instance with given
// AsyncUploader instance and proper mock storage and EDS interfaces.
func createTestBadgerRetryableUploaderWrapper(asyncUploader *AsyncUploader) *BadgerRetryableUploaderWrapper {
	mockBlocksStorage := new(storageMock.Blocks)
	mockBlocksStorage.On("ByID", mock.Anything).Return(nil, nil)

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
	mockExecutionDataDowloader.On("Add", mock.Anything, mock.Anything).Return(flow.ZeroID, nil, nil)
	mockExecutionDataDowloader.On("Download", mock.Anything, mock.Anything).Return(
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
	testComputationResult := &execution.ComputationResult{}
	blockA := unittest.BlockHeaderFixture()
	blockB := unittest.ExecutableBlockFixtureWithParent(nil, blockA)
	testComputationResult.ExecutableBlock = blockB
	return testComputationResult
}
