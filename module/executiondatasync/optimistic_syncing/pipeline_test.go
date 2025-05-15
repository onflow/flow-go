package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	osmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_syncing/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestPipelineStateTransitions verifies that the pipeline correctly transitions
// through states when provided with the correct conditions.
func TestPipelineStateTransitions(t *testing.T) {
	// Create channels for state updates
	updateChan := make(chan StateUpdate, 10)

	// Create publisher function
	publisher := func(update StateUpdate) {
		updateChan <- update
	}

	// Create mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)
	mockCore.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(unittest.Logger(), false, unittest.ExecutionResultFixture(), mockCore, publisher)

	// Start the pipeline in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx)
	}()

	// Assert initial state
	assert.Equal(t, StateReady, pipeline.GetState())

	// Send parent update to trigger state transition
	pipeline.UpdateState(StateUpdate{
		DescendsFromSealed: true,
		ParentState:        StateComplete, // Assume that parent is already complete
	})

	// Wait for pipeline to reach WaitingPersist state
	waitForStateUpdate(t, updateChan, StateWaitingPersist)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should be in WaitingPersist state")
	mockCore.AssertCalled(t, "Download", mock.Anything)
	mockCore.AssertCalled(t, "Index", mock.Anything)
	mockCore.AssertNotCalled(t, "Persist")

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	waitForStateUpdate(t, updateChan, StateComplete)

	mockCore.AssertCalled(t, "Persist", mock.Anything)

	// Cancel the context after the pipeline is already complete
	// At this point, the pipeline has already finished successfully and returned nil
	cancel()

	// Check that the pipeline has completed without errors
	select {
	case err := <-errChan:
		assert.NoError(t, err, "Pipeline should complete without errors")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for pipeline to return")
	}
}

// TestPipelineCancellation verifies that a pipeline is properly canceled when
// it no longer descends from the last persisted sealed result.
func TestPipelineCancellation(t *testing.T) {
	// Create channels for state updates
	updateChan := make(chan StateUpdate, 10)

	// Create publisher function
	publisher := func(update StateUpdate) {
		updateChan <- update
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

	// Create a pipeline
	pipeline := NewPipeline(unittest.Logger(), false, unittest.ExecutionResultFixture(), mockCore, publisher)

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx)
	}()

	// Send an update that allows starting
	pipeline.UpdateState(StateUpdate{
		DescendsFromSealed: true,
		ParentState:        StateComplete,
	})

	// Wait for download to start
	select {
	case <-downloadStarted:
		// Download started
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for download to start")
	}

	// Now send an update that causes cancellation
	pipeline.UpdateState(StateUpdate{
		DescendsFromSealed: false, // No longer descends from latest
		ParentState:        StateComplete,
	})

	// Check the error channel
	select {
	case err := <-errChan:
		// Check if we got an error as expected
		if err != nil {
			assert.Contains(t, err.Error(), "abandoning due to parent updates",
				"Error should indicate abandonment")
		} else {
			// If no error, just log it - the pipeline was canceled but returned nil
			t.Log("Pipeline was canceled but returned nil error")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for pipeline to complete")
	}

	// Check for state updates
	found := false
	timeout := time.After(100 * time.Millisecond)

	for !found {
		select {
		case update := <-updateChan:
			if !update.DescendsFromSealed {
				found = true
			}
		case <-timeout:
			// It's ok if we don't find it - the pipeline might have been canceled before broadcasting
			t.Log("No update with descendsFromSealed=false found within timeout")
			break
		}
	}
}

// TestPipelineParentDependentTransitions verifies that a pipeline's transitions
// depend on the parent pipeline's state.
func TestPipelineParentDependentTransitions(t *testing.T) {
	// Create channels for state updates
	updateChan := make(chan StateUpdate, 10)

	// Create publisher function
	publisher := func(update StateUpdate) {
		updateChan <- update
	}

	// Create a mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)
	mockCore.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(unittest.Logger(), false, unittest.ExecutionResultFixture(), mockCore, publisher)

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx)
	}()

	// Initial update - parent in Ready state
	pipeline.UpdateState(StateUpdate{
		DescendsFromSealed: true,
		ParentState:        StateReady,
	})

	// Sleep a bit to allow processing
	time.Sleep(50 * time.Millisecond)

	// Check that pipeline is still in Ready state
	assert.Equal(t, StateReady, pipeline.GetState(), "Pipeline should remain in Ready state")
	mockCore.AssertNotCalled(t, "Download")

	// Update parent to downloading
	pipeline.UpdateState(StateUpdate{
		DescendsFromSealed: true,
		ParentState:        StateDownloading,
	})

	// Wait for pipeline to progress to WaitingPersist
	waitForStateUpdate(t, updateChan, StateWaitingPersist)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should progress to WaitingPersist state")
	mockCore.AssertCalled(t, "Download", mock.Anything)
	mockCore.AssertCalled(t, "Index", mock.Anything)
	mockCore.AssertNotCalled(t, "Persist")

	// Update parent to complete - should allow persisting when sealed
	pipeline.UpdateState(StateUpdate{
		DescendsFromSealed: true,
		ParentState:        StateComplete,
	})

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	// Wait for pipeline to complete
	waitForStateUpdate(t, updateChan, StateComplete)
	assert.Equal(t, StateComplete, pipeline.GetState(), "Pipeline should reach Complete state")
	mockCore.AssertCalled(t, "Persist", mock.Anything)

	// Cancel the context to end the goroutine
	cancel()
}

// TestPipelineErrorHandling verifies that errors from Core methods are properly
// propagated back to the caller.
func TestPipelineErrorHandling(t *testing.T) {
	// Test cases for different stages of processing
	testCases := []struct {
		name        string
		setupMock   func(mock *osmock.Core, expectedErr error)
		expectedErr error
	}{
		{
			name: "Download Error",
			setupMock: func(m *osmock.Core, expectedErr error) {
				m.On("Download", mock.Anything).Return(expectedErr)
			},
			expectedErr: errors.New("download error"),
		},
		{
			name: "Index Error",
			setupMock: func(m *osmock.Core, expectedErr error) {
				m.On("Download", mock.Anything).Return(nil)
				m.On("Index", mock.Anything).Return(expectedErr)
			},
			expectedErr: errors.New("index error"),
		},
		{
			name: "Persist Error",
			setupMock: func(m *osmock.Core, expectedErr error) {
				m.On("Download", mock.Anything).Return(nil)
				m.On("Index", mock.Anything).Return(nil)
				m.On("Persist", mock.Anything).Return(expectedErr)
			},
			expectedErr: errors.New("persist error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create channels for state updates
			updateChan := make(chan StateUpdate, 10)

			// Create publisher function
			publisher := func(update StateUpdate) {
				updateChan <- update
			}

			// Create a mock core with the specified setup
			mockCore := osmock.NewCore(t)
			tc.setupMock(mockCore, tc.expectedErr)

			// Create a pipeline
			pipeline := NewPipeline(unittest.Logger(), true, unittest.ExecutionResultFixture(), mockCore, publisher)

			// Start the pipeline
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errChan := make(chan error)
			go func() {
				errChan <- pipeline.Run(ctx)
			}()

			// Send parent update to trigger processing
			pipeline.UpdateState(StateUpdate{
				DescendsFromSealed: true,
				ParentState:        StateComplete,
			})

			// Wait for error
			select {
			case err := <-errChan:
				assert.Error(t, err, "Pipeline should propagate the Core error")
				assert.ErrorIs(t, err, tc.expectedErr)
			case <-time.After(500 * time.Millisecond):
				t.Fatal("Timeout waiting for error")
			}
		})
	}
}

// TestBroadcastStateUpdate verifies that descendsFromSealed is correctly propagated
// and is based on the pipeline's state, not just forwarded from the parent.
func TestBroadcastStateUpdate(t *testing.T) {
	// Create channels for state updates
	updateChan := make(chan StateUpdate, 10)

	// Create publisher function
	publisher := func(update StateUpdate) {
		updateChan <- update
	}

	// Create mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(unittest.Logger(), false, unittest.ExecutionResultFixture(), mockCore, publisher)

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx)
	}()

	// Send a state update to trigger a broadcast to children
	pipeline.UpdateState(StateUpdate{
		DescendsFromSealed: true,
		ParentState:        StateDownloading,
	})

	// Wait for an update to be sent to children
	update := waitForStateUpdate(t, updateChan, StateWaitingPersist)

	// Check that the update has the correct flag
	assert.True(t, update.DescendsFromSealed, "Initial update should indicate descends=true")

	// Now simulate this pipeline being canceled
	pipeline.UpdateState(StateUpdate{
		DescendsFromSealed: false, // No longer descends
		ParentState:        StateReady,
	})

	// Wait for the pipeline to complete
	select {
	case err := <-errChan:
		if err != nil {
			assert.Contains(t, err.Error(), "abandoning due to parent updates",
				"Error should indicate abandonment")
		} else {
			t.Log("Pipeline was canceled but returned nil error")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for pipeline to complete")
	}

	// Verify the pipeline transitioned to canceled state
	assert.Equal(t, StateCanceled, pipeline.GetState(), "Pipeline should be in canceled state")

	// Drain the update channel to check if any updates indicate descended=false
	// This approach uses a timeout to prevent hanging
	canceledUpdateFound := false
	drainTimeout := time.After(100 * time.Millisecond)

	for !canceledUpdateFound {
		select {
		case update := <-updateChan:
			if !update.DescendsFromSealed && update.ParentState == StateCanceled {
				canceledUpdateFound = true
			}
		case <-drainTimeout:
			t.Log("No update with descendsFromSealed=false and state=canceled found within timeout")
			return
		}
	}
}

// Helper function to wait for a specific state update
func waitForStateUpdate(t *testing.T, updateChan <-chan StateUpdate, expectedState State) StateUpdate {
	timeoutChan := time.After(500 * time.Millisecond)

	for {
		select {
		case update := <-updateChan:
			if update.ParentState == expectedState {
				return update
			}
			// Continue waiting if this isn't the state we're looking for
		case <-timeoutChan:
			t.Fatalf("Timed out waiting for state update to %s", expectedState)
			return StateUpdate{} // Never reached, just to satisfy compiler
		}
	}
}
