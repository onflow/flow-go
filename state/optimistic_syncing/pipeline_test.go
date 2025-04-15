package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestPipelineStateTransitions verifies that the pipeline correctly transitions
// through states when provided with the correct conditions.
func TestPipelineStateTransitions(t *testing.T) {
	// Create channels for state updates
	childrenUpdateChan := make(chan StateUpdate)

	// Create mock processor
	processor := new(OperationMock)
	processor.On("Download", mock.Anything).Return(nil)
	processor.On("Index", mock.Anything).Return(nil)
	processor.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline
	pipeline := NewPipeline(Config{
		Logger:                  zerolog.Nop(),
		IsSealed:                false,
		ChildrenStateUpdateChan: childrenUpdateChan,
		downloadFunc:            processor.Download,
		indexFunc:               processor.Index,
		persistFunc:             processor.Persist,
	})

	parentUpdateChan := pipeline.GetStateUpdateChan()

	// Start the pipeline in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pipeline.Run(ctx)

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
	processor.AssertCalled(t, "Download", mock.Anything)
	processor.AssertCalled(t, "Index", mock.Anything)
	processor.AssertNotCalled(t, "Persist")

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	waitForStateUpdate(t, childrenUpdateChan, StateComplete)

	processor.AssertCalled(t, "Persist", mock.Anything)
}

// TestPipelineCancellation verifies that a pipeline is properly canceled when
// it no longer descends from the last persisted sealed result.
func TestPipelineCancellation(t *testing.T) {
	// Create channels for state updates
	childrenUpdateChan := make(chan StateUpdate)

	// Set up a download function that signals when it starts and sleeps
	downloadStarted := make(chan struct{})

	// Create a pipeline with a slow download function
	pipeline := NewPipeline(Config{
		Logger:                  zerolog.Nop(),
		IsSealed:                false,
		ChildrenStateUpdateChan: childrenUpdateChan,
		downloadFunc: func(ctx context.Context) error {
			close(downloadStarted) // Signal that download has started

			// Simulate long-running operation
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return nil
			}
		},
	})
	parentUpdateChan := pipeline.GetStateUpdateChan()

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pipeline.Run(ctx)

	// Send an update that allows starting
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateComplete,
	}

	// Wait for download to start
	<-downloadStarted

	// Now send an update that causes cancellation
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: false, // No longer descends from latest
		ParentState:                     StateComplete,
	}

	waitForStateUpdate(t, childrenUpdateChan, StateCanceled)
}

// TestPipelineParentDependentTransitions verifies that a pipeline's transitions
// depend on the parent pipeline's state.
func TestPipelineParentDependentTransitions(t *testing.T) {
	// Create channels for state updates
	childrenUpdateChan := make(chan StateUpdate)

	// Create a pipeline with process functions
	processor := new(OperationMock)
	processor.On("Download", mock.Anything).Return(nil)
	processor.On("Index", mock.Anything).Return(nil)
	processor.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline with initial state
	pipeline := NewPipeline(Config{
		Logger:                  zerolog.Nop(),
		IsSealed:                false,
		ChildrenStateUpdateChan: childrenUpdateChan,
		downloadFunc:            processor.Download,
		indexFunc:               processor.Index,
		persistFunc:             processor.Persist,
	})
	parentUpdateChan := pipeline.GetStateUpdateChan()

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pipeline.Run(ctx)

	// Initial update - parent in Ready state
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateReady,
	}

	// Check that pipeline is still in Ready state
	assert.Equal(t, StateReady, pipeline.GetState(), "Pipeline should remain in Ready state")
	processor.AssertNotCalled(t, "Download")

	// Update parent to downloading
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateDownloading,
	}

	// Wait for pipeline to progress to WaitingPersist
	waitForStateUpdate(t, childrenUpdateChan, StateWaitingPersist)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should progress to WaitingPersist state")
	processor.AssertCalled(t, "Download", mock.Anything)
	processor.AssertCalled(t, "Index", mock.Anything)
	processor.AssertNotCalled(t, "Persist")

	// Update parent to complete - should trigger persisting
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateComplete,
	}

	// Mark the execution result as sealed to trigger persisting
	pipeline.SetSealed()

	// Wait for pipeline to complete
	waitForStateUpdate(t, childrenUpdateChan, StateComplete)
	assert.Equal(t, StateComplete, pipeline.GetState(), "Pipeline should reach Complete state")
	processor.AssertCalled(t, "Persist", mock.Anything)
}

// TestPipelineInitialState verifies that a pipeline can be created with a specific
// initial state and behaves correctly based on that state.
func TestPipelineInitialState(t *testing.T) {
	// Create channels for state updates
	childrenUpdateChan := make(chan StateUpdate)

	// Create mock processor
	processor := new(OperationMock)
	processor.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline with non-default initial state
	pipeline := NewPipeline(Config{
		Logger:                    zerolog.Nop(),
		IsSealed:                  true,
		InitialState:              StateWaitingPersist,
		InitialDescendsFromSealed: true,
		ChildrenStateUpdateChan:   childrenUpdateChan,
		persistFunc:               processor.Persist,
	})
	parentUpdateChan := pipeline.GetStateUpdateChan()

	// Verify initial state is set correctly
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should be initialized in WaitingPersist state")

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pipeline.Run(ctx)

	// Send parent update to trigger persisting
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateComplete,
	}

	// Wait for pipeline to complete
	waitForStateUpdate(t, childrenUpdateChan, StateComplete)
	assert.Equal(t, StateComplete, pipeline.GetState(), "Pipeline should complete")
	processor.AssertCalled(t, "Persist", mock.Anything)
}

// TestBroadcastStateUpdate verifies that descendsFromSealed is correctly propagated
// and is based on the pipeline's state, not just forwarded from the parent.
func TestBroadcastStateUpdate(t *testing.T) {
	// Create channels for state updates
	childrenUpdateChan := make(chan StateUpdate)

	// Create mock processor
	processor := new(OperationMock)
	processor.On("Download", mock.Anything).Return(nil)
	processor.On("Index", mock.Anything).Return(nil)
	processor.On("Persist", mock.Anything).Return(nil)

	// Create a pipeline with initial state
	pipeline := NewPipeline(Config{
		Logger:                    zerolog.Nop(),
		InitialDescendsFromSealed: true,
		ChildrenStateUpdateChan:   childrenUpdateChan,
		downloadFunc:              processor.Download,
		indexFunc:                 processor.Index,
		persistFunc:               processor.Persist,
	})
	parentUpdateChan := pipeline.GetStateUpdateChan()

	// Start the pipeline
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pipeline.Run(ctx)

	// Send a state update to trigger a broadcast to children
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: true,
		ParentState:                     StateDownloading, // This will trigger a state transition
	}

	// Wait for an update to be sent to children
	update := waitForAnyStateUpdate(t, childrenUpdateChan)

	// Check that the update has the correct flag
	assert.True(t, update.DescendsFromLastPersistedSealed, "Initial update should indicate descends=true")

	// The state could be any non-terminal state depending on timing
	assert.NotEqual(t, StateComplete, update.ParentState, "State should not be Complete")
	assert.NotEqual(t, StateCanceled, update.ParentState, "State should not be Canceled")

	// Now simulate this pipeline being cancelled
	parentUpdateChan <- StateUpdate{
		DescendsFromLastPersistedSealed: false, // No longer descends
		ParentState:                     StateReady,
	}

	// Wait for canceled update
	cancelUpdate := waitForStateUpdate(t, childrenUpdateChan, StateCanceled)

	// Check the cancel update
	assert.False(t, cancelUpdate.DescendsFromLastPersistedSealed, "Update after cancellation should indicate descends=false")
	assert.Equal(t, StateCanceled, cancelUpdate.ParentState, "State should be Canceled")
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

// OperationMock is a mock implementation of process functions
type OperationMock struct {
	mock.Mock
}

// Download implements the download process function
func (m *OperationMock) Download(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Index implements the index process function
func (m *OperationMock) Index(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Persist implements the persist process function
func (m *OperationMock) Persist(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
