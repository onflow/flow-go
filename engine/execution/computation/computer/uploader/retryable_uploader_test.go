package uploader

//
//func Test_Upload_invoke(t *testing.T) {
//	wg := sync.WaitGroup{}
//	uploaderCalled := false
//
//	dummyUploader := &DummyUploader{
//		f: func() error {
//			wg.Done()
//			uploaderCalled = true
//			return nil
//		},
//	}
//	asyncUploader := NewAsyncUploader(dummyUploader,
//		1*time.Nanosecond, 1, zerolog.Nop(), &metrics.NoopCollector{})
//
//	testRetryableUploader := createTestBadgerRetryableUploader(asyncUploader)
//	defer testRetryableUploader.Done()
//
//	// nil input - no call to Upload()
//	err := testRetryableUploader.Upload(nil)
//	assert.Error(t, err)
//	assert.False(t, uploaderCalled)
//
//	// non-nil input - Upload() should be called
//	wg.Add(1)
//	testComputationResult := createTestComputationResult()
//	err = testRetryableUploader.Upload(testComputationResult)
//	wg.Wait()
//
//	assert.Nil(t, err)
//	assert.True(t, uploaderCalled)
//}
//
//func Test_RetryUpload(t *testing.T) {
//	wg := sync.WaitGroup{}
//	wg.Add(1)
//	uploaderCalled := false
//	dummyUploader := &DummyUploader{
//		f: func() error {
//			wg.Done()
//			uploaderCalled = true
//			return nil
//		},
//	}
//	asyncUploader := NewAsyncUploader(dummyUploader,
//		1*time.Nanosecond, 1, zerolog.Nop(), &metrics.NoopCollector{})
//
//	testRetryableUploader := createTestBadgerRetryableUploader(asyncUploader)
//	defer testRetryableUploader.Done()
//
//	err := testRetryableUploader.RetryUpload()
//	wg.Wait()
//
//	assert.Nil(t, err)
//	assert.True(t, uploaderCalled)
//}
//
//func Test_AsyncUploaderCallback(t *testing.T) {
//	wgUploadCalleded := sync.WaitGroup{}
//	wgUploadCalleded.Add(1)
//
//	uploader := &DummyUploader{
//		f: func() error {
//			wgUploadCalleded.Done()
//			return nil
//		},
//	}
//	asyncUploader := NewAsyncUploader(uploader,
//		1*time.Nanosecond, 1, zerolog.Nop(), &metrics.NoopCollector{})
//
//	testRetryableUploader := createTestBadgerRetryableUploader(asyncUploader)
//	defer testRetryableUploader.Done()
//
//	testComputationResult := createTestComputationResult()
//	err := testRetryableUploader.Upload(testComputationResult)
//	assert.Nil(t, err)
//
//	wgUploadCalleded.Wait()
//}
//
//// createTestBadgerRetryableUploader() create BadgerRetryableUploader instance with given
//// 	AsyncUploader instance and proper mock storage and EDS interfaces.
//func createTestBadgerRetryableUploader(asyncUploader *AsyncUploader) *BadgerRetryableUploader {
//	mockBlocksStorage := new(storageMock.Blocks)
//	mockBlocksStorage.On("ByID", mock.Anything).Return(nil, nil)
//
//	mockCommitsStorage := new(storageMock.Commits)
//	mockCommitsStorage.On("ByBlockID", mock.Anything).Return(nil, nil)
//
//	mockTransactionResultsStorage := new(storageMock.TransactionResults)
//	mockTransactionResultsStorage.On("ByBlockID", mock.Anything).Return(nil, nil)
//
//	mockComputationResultStorage := new(storageMock.ComputationResultUploadStatus)
//	testID := flow.HashToID([]byte{1, 2, 3})
//	mockComputationResultStorage.On("GetAllIDs").Return([]flow.Identifier{testID}, nil)
//	testComputationResultUploadStatus := false
//	mockComputationResultStorage.On("ByID", testID).Return(testComputationResultUploadStatus, nil)
//	mockComputationResultStorage.On("Upsert", mock.Anything, mock.Anything).Return(nil)
//
//	mockExecutionDataService := new(stateSynchronizationMock.ExecutionDataService)
//	mockExecutionDataService.On("Add", mock.Anything, mock.Anything).Return(flow.ZeroID, nil, nil)
//	mockExecutionDataService.On("Get", mock.Anything, mock.Anything).Return(
//		&stateSynchronization.ExecutionData{
//			BlockID:     flow.ZeroID,
//			Collections: make([]*flow.Collection, 0),
//			Events:      make([]flow.EventsList, 0),
//			TrieUpdates: make([]*ledger.TrieUpdate, 0),
//		}, nil)
//
//	return NewBadgerRetryableUploader(
//		asyncUploader,
//		mockBlocksStorage,
//		mockCommitsStorage,
//		mockTransactionResultsStorage,
//		mockComputationResultStorage,
//		mockExecutionDataService,
//		&metrics.NoopCollector{},
//	)
//}
//
//// createTestComputationResult() creates ComputationResult with valid ExecutableBlock ID
//func createTestComputationResult() *execution.ComputationResult {
//	testComputationResult := &execution.ComputationResult{}
//	blockA := unittest.BlockHeaderFixture()
//	blockB := unittest.ExecutableBlockFixtureWithParent(nil, blockA)
//	testComputationResult.ExecutableBlock = blockB
//	return testComputationResult
//}
