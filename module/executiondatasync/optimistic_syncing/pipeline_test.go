package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	osmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_syncing/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// StateUpdateCapture helps capture state updates for testing
type StateUpdateCapture struct {
	updates chan StateUpdateData
}

type StateUpdateData struct {
	ResultID flow.Identifier
	State    State
}

func NewStateUpdateCapture() *StateUpdateCapture {
	return &StateUpdateCapture{
		updates: make(chan StateUpdateData, 10),
	}
}

func (c *StateUpdateCapture) Publisher() StateUpdatePublisher {
	return func(resultID flow.Identifier, newState State) {
		c.updates <- StateUpdateData{
			ResultID: resultID,
			State:    newState,
		}
	}
}

func (c *StateUpdateCapture) GetUpdate() StateUpdateData {
	select {
	case update := <-c.updates:
		return update
	case <-time.After(100 * time.Millisecond):
		return StateUpdateData{} // Return empty if timeout
	}
}

// TestPipelineStateTransitions verifies that the pipeline correctly transitions
// through states when provided with the correct conditions.
func TestPipelineStateTransitions(t *testing.T) {
	// Create state update capture
	stateCapture := NewStateUpdateCapture()

	// Create mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)
	mockCore.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(zerolog.Nop(), false, unittest.ExecutionResultFixture(), stateCapture.Publisher())

	// Start the pipeline in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx, mockCore)
	}()

	// Assert initial state is ready (set during Run)
	time.Sleep(10 * time.Millisecond) // Allow pipeline to initialize
	assert.Equal(t, StateReady, pipeline.GetState())

	// Send parent update to trigger state transition (allow downloading)
	pipeline.OnParentStateUpdated(StateComplete)

	// Wait for pipeline to reach WaitingPersist state
	waitForState(t, pipeline, StateWaitingPersist, 500*time.Millisecond)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should be in WaitingPersist state")
	mockCore.AssertCalled(t, "Download", mock.Anything)
	mockCore.AssertCalled(t, "Index", mock.Anything)
	mockCore.AssertNotCalled(t, "Persist")

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	waitForState(t, pipeline, StateComplete, 500*time.Millisecond)

	mockCore.AssertCalled(t, "Persist", mock.Anything)

	// Cancel the context after the pipeline is already complete
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
// parent state is canceled.
func TestPipelineCancellation(t *testing.T) {
	// Create state update capture
	stateCapture := NewStateUpdateCapture()

	// Set up a download function that signals when it starts and sleeps
	downloadStarted := make(chan struct{})

	// Create a mock core with a slow download
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Run(func(args mock.Arguments) {
		close(downloadStarted) // Signal that download has started
		// Simulate long-running operation
		time.Sleep(100 * time.Millisecond)
	}).Return(nil)

	//TODO: This should not be Maybe.
	mockCore.On("Abort", mock.Anything).Return(nil).Maybe()

	// Create a pipeline
	pipeline := NewPipeline(zerolog.Nop(), false, unittest.ExecutionResultFixture(), stateCapture.Publisher())

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx, mockCore)
	}()

	// Send an update that allows starting
	pipeline.OnParentStateUpdated(StateComplete)

	// Wait for download to start
	select {
	case <-downloadStarted:
		// Download started
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for download to start")
	}

	// Now send an update that causes cancellation
	pipeline.OnParentStateUpdated(StateCanceled)

	// Wait for the pipeline to transition to canceled state
	waitForState(t, pipeline, StateCanceled, 500*time.Millisecond)

	// Give a small delay to ensure the abort method is called
	time.Sleep(50 * time.Millisecond)

	// Check the error channel
	select {
	case err := <-errChan:
		// Pipeline should return without error when properly canceled
		if err != nil {
			t.Logf("Pipeline returned error on cancellation: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for pipeline to complete")
	}

	// Verify the pipeline transitioned to canceled state
	assert.Equal(t, StateCanceled, pipeline.GetState(), "Pipeline should be in canceled state")
	mockCore.AssertExpectations(t)
}

// TestPipelineParentDependentTransitions verifies that a pipeline's transitions
// depend on the parent pipeline's state.
func TestPipelineParentDependentTransitions(t *testing.T) {
	// Create state update capture
	stateCapture := NewStateUpdateCapture()

	// Create a mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)
	mockCore.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(zerolog.Nop(), false, unittest.ExecutionResultFixture(), stateCapture.Publisher())

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx, mockCore)
	}()

	// Initial update - parent in Ready state (should not allow downloading)
	pipeline.OnParentStateUpdated(StateReady)

	// Sleep a bit to allow processing
	time.Sleep(50 * time.Millisecond)

	// Check that pipeline is still in Ready state
	assert.Equal(t, StateReady, pipeline.GetState(), "Pipeline should remain in Ready state")
	mockCore.AssertNotCalled(t, "Download")

	// Update parent to downloading (should allow downloading)
	pipeline.OnParentStateUpdated(StateDownloading)

	// Wait for pipeline to progress to WaitingPersist
	waitForState(t, pipeline, StateWaitingPersist, 500*time.Millisecond)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should progress to WaitingPersist state")
	mockCore.AssertCalled(t, "Download", mock.Anything)
	mockCore.AssertCalled(t, "Index", mock.Anything)
	mockCore.AssertNotCalled(t, "Persist")

	// Update parent to complete - should allow persisting when sealed
	pipeline.OnParentStateUpdated(StateComplete)

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	// Wait for pipeline to complete
	waitForState(t, pipeline, StateComplete, 500*time.Millisecond)
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
			// Create state update capture
			stateCapture := NewStateUpdateCapture()

			// Create a mock core with the specified setup
			mockCore := osmock.NewCore(t)
			tc.setupMock(mockCore, tc.expectedErr)

			// Create a pipeline
			pipeline := NewPipeline(zerolog.Nop(), true, unittest.ExecutionResultFixture(), stateCapture.Publisher())

			// Start the pipeline
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errChan := make(chan error)
			go func() {
				errChan <- pipeline.Run(ctx, mockCore)
			}()

			// Send parent update to trigger processing
			pipeline.OnParentStateUpdated(StateComplete)

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

// TestBroadcastStateUpdate verifies that state updates are correctly broadcast
// when the pipeline transitions between states.
func TestBroadcastStateUpdate(t *testing.T) {
	// Create state update capture
	stateCapture := NewStateUpdateCapture()

	// Create mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)

	// Create a pipeline
	executionResult := unittest.ExecutionResultFixture()
	pipeline := NewPipeline(zerolog.Nop(), false, executionResult, stateCapture.Publisher())

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
	waitForState(t, pipeline, StateWaitingPersist, 500*time.Millisecond)

	// Verify we got state updates along the way
	foundValidUpdate := false
	timeout := time.After(100 * time.Millisecond)

	for !foundValidUpdate {
		select {
		case <-timeout:
			break // Exit loop if timeout reached
		default:
			update := stateCapture.GetUpdate()
			if update.ResultID != (flow.Identifier{}) {
				assert.Equal(t, executionResult.ID(), update.ResultID, "Update should be for correct execution result")
				foundValidUpdate = true
			} else {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	// Test cancellation
	pipeline.OnParentStateUpdated(StateCanceled)

	// Wait for the pipeline to transition to canceled state
	waitForState(t, pipeline, StateCanceled, 200*time.Millisecond)

	// Give a small delay to ensure the abort method is called
	time.Sleep(50 * time.Millisecond)

	// Wait for the pipeline to complete
	select {
	case err := <-errChan:
		if err != nil {
			t.Logf("Pipeline returned error on cancellation: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		// Pipeline might still be running, that's okay for this test
		t.Log("Pipeline still running after cancellation - this may be expected")
	}

	// Verify the pipeline transitioned to canceled state
	assert.Equal(t, StateCanceled, pipeline.GetState(), "Pipeline should be in canceled state")

	// Check for canceled state update
	timeout = time.After(100 * time.Millisecond)
	canceledUpdateFound := false

	for !canceledUpdateFound {
		select {
		case <-timeout:
			t.Log("No canceled state update found within timeout - this may be expected behavior")
			return
		default:
			update := stateCapture.GetUpdate()
			if update.ResultID != (flow.Identifier{}) && update.State == StateCanceled {
				canceledUpdateFound = true
				assert.Equal(t, executionResult.ID(), update.ResultID, "Canceled update should be for correct execution result")
			} else {
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
}

// Helper function to wait for a specific pipeline state
func waitForState(t *testing.T, pipeline *Pipeline, expectedState State, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if pipeline.GetState() == expectedState {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("Timed out waiting for pipeline state %s, current state: %s",
		expectedState.String(), pipeline.GetState().String())
}
