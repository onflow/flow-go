package optimistic_sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	osmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestPipelineStateTransitions verifies that the pipeline correctly transitions
// through states when provided with the correct conditions.
func TestPipelineStateTransitions(t *testing.T) {
	// Create channels for state updates
	updateChan := make(chan State, 10)

	// Create publisher function
	publisher := func(state State) {
		updateChan <- state
	}

	// Create mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index").Return(nil)
	mockCore.On("Persist").Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(zerolog.Nop(), false, unittest.ExecutionResultFixture(), mockCore, publisher)

	// Start the pipeline in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx, mockCore)
	}()

	// Assert initial state
	assert.Equal(t, StatePending, pipeline.GetState())

	// Send parent update to trigger state transition
	pipeline.OnParentStateUpdated(StateComplete) // Assume that parent is already complete

	// Wait for pipeline to reach WaitingPersist state
	waitForStateUpdate(t, pipeline, updateChan, StateWaitingPersist)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should be in WaitingPersist state")
	mockCore.AssertCalled(t, "Download", mock.Anything)
	mockCore.AssertCalled(t, "Index")
	mockCore.AssertNotCalled(t, "Persist")

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	waitForStateUpdate(t, pipeline, updateChan, StateComplete)

	mockCore.AssertCalled(t, "Persist")

	// Cancel the context after the pipeline is already complete
	// At this point, the pipeline has already finished successfully and returned nil
	cancel()

	// Check that the pipeline has completed without errors
	waitForError(t, errChan, nil)
}

// TestPipelineCancellation verifies that a pipeline is properly abandoned when
// the parent pipeline is abandoned.
func TestPipelineCancellation(t *testing.T) {
	// Create channels for state updates
	updateChan := make(chan State, 10)

	// Create publisher function
	publisher := func(state State) {
		updateChan <- state
	}

	// Set up a download function that signals when it starts and sleeps
	downloadStarted := make(chan struct{})

	// Create a mock core with a slow download
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Run(func(args mock.Arguments) {
		close(downloadStarted) // Signal that download has started

		// Simulate long-running operation
		time.Sleep(100 * time.Millisecond)
	}).Return(nil)
	mockCore.On("Abandon").Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(zerolog.Nop(), false, unittest.ExecutionResultFixture(), mockCore, publisher)

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx, mockCore)
	}()

	// Send an update that allows starting
	pipeline.OnParentStateUpdated(StateDownloading)

	// Wait for download to start
	unittest.RequireCloseBefore(t, downloadStarted, 500*time.Millisecond, "Timeout waiting for download to start")

	// Now send an update that causes abandonment
	pipeline.OnParentStateUpdated(StateAbandoned)

	waitForError(t, errChan, context.Canceled)
	waitForStateUpdate(t, pipeline, updateChan, StateAbandoned)
}

// TestPipelineParentDependentTransitions verifies that a pipeline's transitions
// depend on the parent pipeline's state.
func TestPipelineParentDependentTransitions(t *testing.T) {
	// Create channels for state updates
	updateChan := make(chan State, 10)

	// Create publisher function
	publisher := func(state State) {
		updateChan <- state
	}

	// Create a mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index").Return(nil)
	mockCore.On("Persist").Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(zerolog.Nop(), false, unittest.ExecutionResultFixture(), mockCore, publisher)

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx, mockCore)
	}()

	// Initial update - parent in Ready state
	pipeline.OnParentStateUpdated(StateReady)

	// Sleep a bit to allow processing
	time.Sleep(50 * time.Millisecond)

	// Check that pipeline is still in Ready state
	assert.Equal(t, StateReady, pipeline.GetState(), "Pipeline should remain in Ready state")
	mockCore.AssertNotCalled(t, "Download")

	// Update parent to downloading
	pipeline.OnParentStateUpdated(StateDownloading)

	// Wait for pipeline to progress to WaitingPersist
	waitForStateUpdate(t, pipeline, updateChan, StateWaitingPersist)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should progress to WaitingPersist state")
	mockCore.AssertCalled(t, "Download", mock.Anything)
	mockCore.AssertCalled(t, "Index")
	mockCore.AssertNotCalled(t, "Persist")

	// Update parent to complete - should allow persisting when sealed
	pipeline.OnParentStateUpdated(StateComplete)

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	// Wait for pipeline to complete
	waitForStateUpdate(t, pipeline, updateChan, StateComplete)
	assert.Equal(t, StateComplete, pipeline.GetState(), "Pipeline should reach Complete state")
	mockCore.AssertCalled(t, "Persist")

	// Cancel the context to end the goroutine
	cancel()
}

// TestPipelineErrorHandling verifies that errors from Core methods are properly
// propagated back to the caller.
func TestPipelineErrorHandling(t *testing.T) {
	// Test cases for different stages of processing
	testCases := []struct {
		name        string
		setupMock   func(p *PipelineImpl, mock *osmock.Core, expectedErr error)
		expectedErr error
	}{
		{
			name: "Download Error",
			setupMock: func(p *PipelineImpl, m *osmock.Core, expectedErr error) {
				m.On("Download", mock.Anything).Return(expectedErr)
			},
			expectedErr: errors.New("download error"),
		},
		{
			name: "Index Error",
			setupMock: func(p *PipelineImpl, m *osmock.Core, expectedErr error) {
				m.On("Download", mock.Anything).Return(nil)
				m.On("Index").Return(expectedErr)
			},
			expectedErr: errors.New("index error"),
		},
		{
			name: "Persist Error",
			setupMock: func(p *PipelineImpl, m *osmock.Core, expectedErr error) {
				m.On("Download", mock.Anything).Return(nil)
				m.On("Index").Run(func(args mock.Arguments) {
					p.OnParentStateUpdated(StateComplete)
					p.SetSealed()
				}).Return(nil)
				m.On("Persist").Return(expectedErr)
			},
			expectedErr: errors.New("persist error"),
		},
		{
			name: "Abandon Error",
			setupMock: func(p *PipelineImpl, m *osmock.Core, expectedErr error) {
				m.On("Download", mock.Anything).Run(func(args mock.Arguments) {
					p.OnParentStateUpdated(StateAbandoned)
				}).Return(nil)
				m.On("Abandon").Return(expectedErr)
			},
			expectedErr: errors.New("abandon error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create channels for state updates
			updateChan := make(chan State, 10)

			// Create publisher function
			publisher := func(state State) {
				updateChan <- state
			}

			// Create a mock core with the specified setup
			mockCore := osmock.NewCore(t)

			// Create a pipeline
			pipeline := NewPipeline(zerolog.Nop(), false, unittest.ExecutionResultFixture(), mockCore, publisher)

			tc.setupMock(pipeline, mockCore, tc.expectedErr)

			// Start the pipeline
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errChan := make(chan error)
			go func() {
				errChan <- pipeline.Run(ctx, mockCore)
			}()

			// Send parent update to trigger processing
			pipeline.OnParentStateUpdated(StateDownloading)

			waitForError(t, errChan, tc.expectedErr)
		})
	}
}

// TestBroadcastStateUpdate verifies that state updates are properly propagated
// to child pipelines and that abandonment is handled correctly.
func TestBroadcastStateUpdate(t *testing.T) {
	// Create channels for state updates
	updateChan := make(chan State, 10)

	// Create publisher function
	publisher := func(state State) {
		updateChan <- state
	}

	// Create mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index").Return(nil)
	mockCore.On("Abandon").Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(zerolog.Nop(), false, unittest.ExecutionResultFixture(), mockCore, publisher)

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx, mockCore)
	}()

	// Send a state update to trigger processing
	pipeline.OnParentStateUpdated(StateDownloading)

	// Wait for pipeline to reach WaitingPersist state
	waitForStateUpdate(t, pipeline, updateChan, StateWaitingPersist)

	// Now simulate parent abandonment
	pipeline.OnParentStateUpdated(StateAbandoned)

	// Wait for the pipeline to complete and verify it transitioned to abandoned state
	waitForError(t, errChan, context.Canceled)
	waitForStateUpdate(t, pipeline, updateChan, StateAbandoned)
}

// waitForStateUpdate waits for a specific state update to occur or timeout after 500ms.
func waitForStateUpdate(t *testing.T, pipeline *PipelineImpl, updateChan <-chan State, expectedState State) {
	unittest.RequireReturnsBefore(t, func() {
		for {
			update := <-updateChan
			if update == expectedState {
				assert.Equal(t, expectedState, pipeline.GetState())
				return
			}
		}
	}, 500*time.Millisecond, "Timeout waiting for state update")
}

// waitForError waits for an error to occur or timeout after 500ms.
func waitForError(t *testing.T, errChan <-chan error, expectedErr error) {
	unittest.RequireReturnsBefore(t, func() {
		err := <-errChan
		if expectedErr == nil {
			assert.NoError(t, err, "Pipeline should complete without errors")
		} else {
			assert.ErrorIs(t, err, expectedErr)
		}
	}, 500*time.Millisecond, "Timeout waiting for error")
}
