package uploader

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/ingestion/uploader/mock"
	executionUnittest "github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

func TestManagerSetEnabled(t *testing.T) {
	uploadMgr := NewManager(trace.NewNoopTracer())
	assert.False(t, uploadMgr.Enabled())

	uploadMgr.SetEnabled(true)
	assert.True(t, uploadMgr.Enabled())

	uploadMgr.SetEnabled(false)
	assert.False(t, uploadMgr.Enabled())
}

func TestManagerUploadsWithAllUploaders(t *testing.T) {
	result := executionUnittest.ComputationResultFixture(
		t,
		flow.ZeroID,
		[][]flow.Identifier{
			{flow.ZeroID},
			{flow.ZeroID},
			{flow.ZeroID},
		})

	t.Run("uploads with no errors", func(t *testing.T) {
		uploader1 := mock.NewUploader(t)
		uploader1.On("Upload", result).Return(nil).Once()

		uploader2 := mock.NewUploader(t)
		uploader2.On("Upload", result).Return(nil).Once()

		uploader3 := mock.NewUploader(t)
		uploader3.On("Upload", result).Return(nil).Once()

		uploadMgr := NewManager(trace.NewNoopTracer())
		uploadMgr.AddUploader(uploader1)
		uploadMgr.AddUploader(uploader2)
		uploadMgr.AddUploader(uploader3)

		err := uploadMgr.Upload(context.Background(), result)
		assert.NoError(t, err)
	})

	t.Run("uploads and returns error", func(t *testing.T) {
		uploader2Err := fmt.Errorf("uploader 2 error")

		uploader1 := mock.NewUploader(t)
		uploader1.On("Upload", result).Return(nil).Once()

		uploader2 := mock.NewUploader(t)
		uploader2.On("Upload", result).Return(uploader2Err).Once()

		uploader3 := mock.NewUploader(t)
		uploader3.On("Upload", result).Return(nil).Once()

		uploadMgr := NewManager(trace.NewNoopTracer())
		uploadMgr.AddUploader(uploader1)
		uploadMgr.AddUploader(uploader2)
		uploadMgr.AddUploader(uploader3)

		err := uploadMgr.Upload(context.Background(), result)
		assert.ErrorIs(t, err, uploader2Err)
	})
}

func TestRetryableUploader(t *testing.T) {
	testRetryableUploader := new(FakeRetryableUploader)

	uploadMgr := NewManager(trace.NewNoopTracer())
	uploadMgr.AddUploader(testRetryableUploader)

	err := uploadMgr.RetryUploads()
	assert.Nil(t, err)

	require.True(t, testRetryableUploader.RetryUploadCalled())
}

// FakeRetryableUploader is one RetryableUploader for testing purposes.
type FakeRetryableUploader struct {
	RetryableUploaderWrapper
	retryUploadCalled bool
}

func (f *FakeRetryableUploader) Upload(_ *execution.ComputationResult) error {
	return nil
}

func (f *FakeRetryableUploader) RetryUpload() error {
	f.retryUploadCalled = true
	return nil
}

func (f *FakeRetryableUploader) RetryUploadCalled() bool {
	return f.retryUploadCalled
}
