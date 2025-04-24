package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	osmock "github.com/onflow/flow-go/state/optimistic_syncing/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestPipelineStateTransitions verifies that the pipeline correctly transitions
// through states when provided with the correct conditions.
func TestPipelineStateTransitions(t *testing.T) {
	// Create channels for state updates
	childrenUpdateChan := make(chan StateUpdate, 10)

	// Create mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)
	mockCore.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(Config{
		Logger:                   zerolog.Nop(),
		IsSealed:                 false,
		ExecutionResult:          unittest.ExecutionResultFixture(),
		Core:                     mockCore,
		ChildrenStateUpdateChans: []chan<- StateUpdate{childrenUpdateChan},
	})

	parentUpdateChan := pipeline.GetStateUpdateChan()

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
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateComplete, // Assume that parent is already complete
	}

	// Wait for pipeline to reach WaitingPersist state
	waitForStateUpdate(t, childrenUpdateChan, StateWaitingPersist)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should be in WaitingPersist state")
	mockCore.AssertCalled(t, "Download", mock.Anything)
	mockCore.AssertCalled(t, "Index", mock.Anything)
	mockCore.AssertNotCalled(t, "Persist")

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	waitForStateUpdate(t, childrenUpdateChan, StateComplete)

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
	childrenUpdateChan := make(chan StateUpdate, 10)

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
	pipeline := NewPipeline(Config{
		Logger:                   zerolog.Nop(),
		IsSealed:                 false,
		ExecutionResult:          unittest.ExecutionResultFixture(),
		Core:                     mockCore,
		ChildrenStateUpdateChans: []chan<- StateUpdate{childrenUpdateChan},
	})

	parentUpdateChan := pipeline.GetStateUpdateChan()

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx)
	}()

	// Send an update that allows starting
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateComplete,
	}

	// Wait for download to start
	select {
	case <-downloadStarted:
		// Download started
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for download to start")
	}

	// Now send an update that causes cancellation
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: false, // No longer descends from latest
		ParentState:                     StateComplete,
	}

	waitForStateUpdate(t, childrenUpdateChan, StateCanceled)

	// Check for returned error
	select {
	case err := <-errChan:
		assert.NoError(t, err, "Pipeline should not return error")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for pipeline to return error")
	}
}

// TestPipelineParentDependentTransitions verifies that a pipeline's transitions
// depend on the parent pipeline's state.
func TestPipelineParentDependentTransitions(t *testing.T) {
	// Create channels for state updates
	childrenUpdateChan := make(chan StateUpdate, 10)

	// Create a mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)
	mockCore.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(Config{
		Logger:                   zerolog.Nop(),
		IsSealed:                 false,
		ExecutionResult:          unittest.ExecutionResultFixture(),
		Core:                     mockCore,
		ChildrenStateUpdateChans: []chan<- StateUpdate{childrenUpdateChan},
	})

	parentUpdateChan := pipeline.GetStateUpdateChan()

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx)
	}()

	// Initial update - parent in Ready state
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateReady,
	}

	// Sleep a bit to allow processing
	time.Sleep(50 * time.Millisecond)

	// Check that pipeline is still in Ready state
	assert.Equal(t, StateReady, pipeline.GetState(), "Pipeline should remain in Ready state")
	mockCore.AssertNotCalled(t, "Download")

	// Update parent to downloading
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateDownloading,
	}

	// Wait for pipeline to progress to WaitingPersist
	waitForStateUpdate(t, childrenUpdateChan, StateWaitingPersist)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should progress to WaitingPersist state")
	mockCore.AssertCalled(t, "Download", mock.Anything)
	mockCore.AssertCalled(t, "Index", mock.Anything)
	mockCore.AssertNotCalled(t, "Persist")

	// Update parent to complete - should allow persisting when sealed
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateComplete,
	}

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	// Wait for pipeline to complete
	waitForStateUpdate(t, childrenUpdateChan, StateComplete)
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
		name      string
		setupMock func(mock *osmock.Core)
	}{
		{
			name: "Download Error",
			setupMock: func(m *osmock.Core) {
				m.On("Download", mock.Anything).Return(assert.AnError)
			},
		},
		{
			name: "Index Error",
			setupMock: func(m *osmock.Core) {
				m.On("Download", mock.Anything).Return(nil)
				m.On("Index", mock.Anything).Return(assert.AnError)
			},
		},
		{
			name: "Persist Error",
			setupMock: func(m *osmock.Core) {
				m.On("Download", mock.Anything).Return(nil)
				m.On("Index", mock.Anything).Return(nil)
				m.On("Persist", mock.Anything).Return(assert.AnError)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create channels for state updates
			childrenUpdateChan := make(chan StateUpdate, 10)

			// Create a mock core with the specified setup
			mockCore := new(osmock.Core)
			tc.setupMock(mockCore)

			// Create a pipeline
			pipeline := NewPipeline(Config{
				Logger:                   zerolog.Nop(),
				IsSealed:                 true, // Set to true to allow testing persist
				ExecutionResult:          unittest.ExecutionResultFixture(),
				Core:                     mockCore,
				ChildrenStateUpdateChans: []chan<- StateUpdate{childrenUpdateChan},
			})

			parentUpdateChan := pipeline.GetStateUpdateChan()

			// Start the pipeline
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errChan := make(chan error)
			go func() {
				errChan <- pipeline.Run(ctx)
			}()

			// Send parent update to trigger processing
			parentUpdateChan <- StateUpdate{
				DescendsFromLastPersistedSealed: true,
				ParentState:                     StateComplete,
			}

			// Wait for error
			select {
			case err := <-errChan:
				assert.ErrorIs(t, err, assert.AnError, "Pipeline should propagate the Core error")
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
	childrenUpdateChan := make(chan StateUpdate, 10)

	// Create mock core
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(Config{
		Logger:                   zerolog.Nop(),
		IsSealed:                 false,
		ExecutionResult:          unittest.ExecutionResultFixture(),
		Core:                     mockCore,
		ChildrenStateUpdateChans: []chan<- StateUpdate{childrenUpdateChan},
	})

	parentUpdateChan := pipeline.GetStateUpdateChan()

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx)
	}()

	// Send a state update to trigger a broadcast to children
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateDownloading,
	}

	// Wait for an update to be sent to children
	update := waitForAnyStateUpdate(t, childrenUpdateChan)

	// Check that the update has the correct flag
	assert.True(t, update.DescendsFromLastPersistedSealed, "Initial update should indicate descends=true")

	// Now simulate this pipeline being cancelled
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: false, // No longer descends
		ParentState:                     StateReady,
	}

	// Wait for the pipeline to return an error
	select {
	case err := <-errChan:
		assert.Equal(t, context.Canceled, err, "Pipeline should return context.Canceled")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for pipeline to return error")
	}

	// Drain the channel and check if any update has descendsFromSealed=false
	found := false
	timeout := time.After(500 * time.Millisecond)

	for !found {
		select {
		case update := <-childrenUpdateChan:
			if !update.DescendsFromLastPersistedSealed {
				found = true
			}
		case <-timeout:
			t.Fatal("Timeout waiting for update with descendsFromSealed=false")
		}
	}

	assert.True(t, found, "Should have found an update with descendsFromSealed=false")

	cancel()
}

// TestMultipleChildren verifies that state updates are correctly sent to multiple children
func TestMultipleChildren(t *testing.T) {
	// Create channels for state updates
	childUpdateChan1 := make(chan StateUpdate, 10)
	childUpdateChan2 := make(chan StateUpdate, 10)

	// Create mock core - expect both Download and Index since the pipeline will
	// progress through these states
	mockCore := osmock.NewCore(t)
	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index", mock.Anything).Return(nil)

	// Create a pipeline with multiple children
	pipeline := NewPipeline(Config{
		Logger:                   zerolog.Nop(),
		IsSealed:                 false,
		ExecutionResult:          unittest.ExecutionResultFixture(),
		Core:                     mockCore,
		ChildrenStateUpdateChans: []chan<- StateUpdate{childUpdateChan1, childUpdateChan2},
	})

	parentUpdateChan := pipeline.GetStateUpdateChan()

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx)
	}()

	// Send parent update to trigger state transition
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateComplete,
	}

	// Wait for updates to both children
	update1 := waitForAnyStateUpdate(t, childUpdateChan1)
	update2 := waitForAnyStateUpdate(t, childUpdateChan2)

	// Verify both children received the same update
	assert.Equal(t, update1.ParentState, update2.ParentState, "Both children should receive the same state")
	assert.Equal(t, update1.DescendsFromLastPersistedSealed, update2.DescendsFromLastPersistedSealed,
		"Both children should receive the same descends status")

	// Cancel the context to end the goroutine
	cancel()
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

// Helper function to wait for any state update
func waitForAnyStateUpdate(t *testing.T, updateChan <-chan StateUpdate) StateUpdate {
	timeoutChan := time.After(500 * time.Millisecond)

	select {
	case update := <-updateChan:
		return update
	case <-timeoutChan:
		t.Fatal("Timed out waiting for any state update")
		return StateUpdate{} // Never reached, just to satisfy compiler
	}
}
