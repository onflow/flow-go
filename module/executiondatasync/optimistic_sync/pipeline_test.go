package optimistic_sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	osmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestPipelineStateTransitions verifies that the pipeline correctly transitions
// through states when provided with the correct conditions.
func TestPipelineStateTransitions(t *testing.T) {
	pipeline, mockCore, updateChan, parent := createPipeline(t)

	pipeline.SetSealed()
	parent.UpdateState(StateComplete, pipeline)

	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index").Return(nil)
	mockCore.On("Persist").Return(nil)

	assert.Equal(t, StatePending, pipeline.GetState(), "Pipeline should start in Pending state")

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(context.Background(), mockCore, parent.GetState())
	}()

	// Wait for pipeline to reach WaitingPersist state
	expectedStates := []State{StateProcessing, StateWaitingPersist, StateComplete}
	waitForStateUpdates(t, updateChan, errChan, expectedStates...)
	assert.Equal(t, StateComplete, pipeline.GetState(), "Pipeline should be in Complete state")

	// Run should complete without error
	waitForError(t, errChan, nil)
}

// TestPipelineParentDependentTransitions verifies that a pipeline's transitions
// depend on the parent pipeline's state.
func TestPipelineParentDependentTransitions(t *testing.T) {
	pipeline, mockCore, updateChan, parent := createPipeline(t)

	mockCore.On("Download", mock.Anything).Return(nil)
	mockCore.On("Index").Return(nil)
	mockCore.On("Persist").Return(nil)

	assert.Equal(t, StatePending, pipeline.GetState(), "Pipeline should start in Pending state")

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(context.Background(), mockCore, parent.GetState())
	}()

	// Initial update - parent in Ready state
	parent.UpdateState(StatePending, pipeline)

	// Check that pipeline remains in Ready state
	waitNeverStateUpdate(t, updateChan, errChan)
	assert.Equal(t, StatePending, pipeline.GetState(), "Pipeline should start in Ready state")
	mockCore.AssertNotCalled(t, "Download")

	// Update parent to downloading
	parent.UpdateState(StateProcessing, pipeline)

	// Pipeline should now progress to WaitingPersist state and stop
	expectedStates := []State{StateProcessing, StateWaitingPersist}
	waitForStateUpdates(t, updateChan, errChan, expectedStates...)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should progress to WaitingPersist state")
	mockCore.AssertCalled(t, "Download", mock.Anything)
	mockCore.AssertCalled(t, "Index")
	mockCore.AssertNotCalled(t, "Persist")

	waitNeverStateUpdate(t, updateChan, errChan)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should remain in WaitingPersist state")

	// Update parent to complete - should allow persisting when sealed
	parent.UpdateState(StateComplete, pipeline)

	// this alone should not allow the pipeline to progress to any other state
	waitNeverStateUpdate(t, updateChan, errChan)
	assert.Equal(t, StateWaitingPersist, pipeline.GetState(), "Pipeline should remain in WaitingPersist state")

	// Mark the execution result as sealed, this should allow the pipeline to progress to Complete state
	pipeline.SetSealed()

	// Wait for pipeline to complete
	expectedStates = []State{StateComplete}
	waitForStateUpdates(t, updateChan, errChan, expectedStates...)
	assert.Equal(t, StateComplete, pipeline.GetState(), "Pipeline should reach Complete state")
	mockCore.AssertCalled(t, "Persist")

	// Run should complete without error
	waitForError(t, errChan, nil)
}

// TestParentAbandoned verifies that a pipeline is properly abandoned when
// the parent pipeline is abandoned.
func TestAbandoned(t *testing.T) {
	t.Run("starts already abandoned", func(t *testing.T) {
		pipeline, mockCore, updateChan, parent := createPipeline(t)

		mockCore.On("Abandon").Return(nil)

		pipeline.Abandon()

		errChan := make(chan error)
		go func() {
			errChan <- pipeline.Run(context.Background(), mockCore, parent.GetState())
		}()

		// first state must be abandoned
		waitForStateUpdates(t, updateChan, errChan, StateAbandoned)

		// Run should complete without error
		waitForError(t, errChan, nil)
	})

	// Test cases abandoning during different stages of processing
	testCases := []struct {
		name           string
		setupMock      func(*PipelineImpl, *mockStateProvider, *osmock.Core)
		expectedStates []State
	}{
		{
			name: "Abandon during download",
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Run(func(args mock.Arguments) {
					pipeline.Abandon()

					ctx := args[0].(context.Context)
					unittest.RequireCloseBefore(t, ctx.Done(), 500*time.Millisecond, "Abandon should cause context to be canceled")
				}).Return(func(ctx context.Context) error {
					return ctx.Err()
				})
			},
			expectedStates: []State{StateProcessing, StateAbandoned},
		},
		{
			name: "Parent abandoned during download",
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Run(func(args mock.Arguments) {
					parent.UpdateState(StateAbandoned, pipeline)

					ctx := args[0].(context.Context)
					unittest.RequireCloseBefore(t, ctx.Done(), 500*time.Millisecond, "Abandon should cause context to be canceled")
				}).Return(func(ctx context.Context) error {
					return ctx.Err()
				})
			},
			expectedStates: []State{StateProcessing, StateAbandoned},
		},
		{
			name: "Abandon during index",
			// Note: indexing will complete, and the pipeline will transition to waiting persist
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Run(func(args mock.Arguments) {
					pipeline.Abandon()
				}).Return(nil)
			},
			expectedStates: []State{StateProcessing, StateAbandoned},
		},
		{
			name: "Parent abandoned during index",
			// Note: indexing will complete, and the pipeline will transition to waiting persist
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Run(func(args mock.Arguments) {
					parent.UpdateState(StateAbandoned, pipeline)
				}).Return(nil)
			},
			expectedStates: []State{StateProcessing, StateAbandoned},
		},
		{
			name: "Abandon during waiting to persist",
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Run(func(args mock.Arguments) {
					go func() {
						time.Sleep(100 * time.Millisecond)
						pipeline.Abandon()
					}()
				}).Return(nil)
			},
			expectedStates: []State{StateProcessing, StateWaitingPersist, StateAbandoned},
		},
		{
			name: "Parent abandoned during waiting to persist",
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Run(func(args mock.Arguments) {
					go func() {
						time.Sleep(100 * time.Millisecond)
						parent.UpdateState(StateAbandoned, pipeline)
					}()
				}).Return(nil)
			},
			expectedStates: []State{StateProcessing, StateWaitingPersist, StateAbandoned},
		},
		// Note: it does not make sense to abandon during persist, since it will only be run when:
		// 1. the parent is already complete
		// 2. the pipeline's result is sealed
		// At that point, there are no conditions that would cause the pipeline to transition to any other state
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pipeline, mockCore, updateChan, parent := createPipeline(t)
			tc.setupMock(pipeline, parent, mockCore)

			mockCore.On("Abandon").Return(nil)

			errChan := make(chan error)
			go func() {
				errChan <- pipeline.Run(context.Background(), mockCore, parent.GetState())
			}()

			// Send parent update to start processing
			parent.UpdateState(StateProcessing, pipeline)

			waitForStateUpdates(t, updateChan, errChan, tc.expectedStates...)

			waitForError(t, errChan, nil)
		})
	}
}

// TestPipelineContextCancellation tests the Run method's context cancelation behavior during different stages of processing
func TestPipelineContextCancellation(t *testing.T) {
	// Test cases for different stages of processing
	testCases := []struct {
		name      string
		setupMock func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) context.Context
	}{
		{
			name: "Cancel before download starts",
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				// no Core methods called
				return ctx
			},
		},
		{
			name: "Cancel during download",
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				mockCore.On("Download", mock.Anything).Run(func(args mock.Arguments) {
					cancel()
					pipelineCtx := args[0].(context.Context)
					unittest.RequireCloseBefore(t, pipelineCtx.Done(), 500*time.Millisecond, "Abandon should cause context to be canceled")
				}).Return(func(pipelineCtx context.Context) error {
					return pipelineCtx.Err()
				})
				return ctx
			},
		},
		{
			name: "Cancel between steps",
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core) context.Context {
				ctx, cancel := context.WithCancel(context.Background())

				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Run(func(args mock.Arguments) {
					cancel()
				}).Return(nil)

				return ctx
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pipeline, mockCore, _, parent := createPipeline(t)

			parent.UpdateState(StateComplete, pipeline)
			pipeline.SetSealed()

			ctx := tc.setupMock(pipeline, parent, mockCore)

			errChan := make(chan error)
			go func() {
				errChan <- pipeline.Run(ctx, mockCore, parent.GetState())
			}()

			waitForError(t, errChan, context.Canceled)
		})
	}
}

// TestPipelineErrorHandling verifies that errors from Core methods are properly
// propagated back to the caller.
func TestPipelineErrorHandling(t *testing.T) {
	// Test cases for different stages of processing
	testCases := []struct {
		name           string
		setupMock      func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core, expectedErr error)
		expectedErr    error
		expectedStates []State
	}{
		{
			name: "Download Error",
			setupMock: func(pipeline *PipelineImpl, _ *mockStateProvider, mockCore *osmock.Core, expectedErr error) {
				mockCore.On("Download", mock.Anything).Return(expectedErr)
			},
			expectedErr:    errors.New("download error"),
			expectedStates: []State{StateProcessing},
		},
		{
			name: "Index Error",
			setupMock: func(pipeline *PipelineImpl, _ *mockStateProvider, mockCore *osmock.Core, expectedErr error) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Return(expectedErr)
			},
			expectedErr:    errors.New("index error"),
			expectedStates: []State{StateProcessing},
		},
		{
			name: "Persist Error",
			setupMock: func(pipeline *PipelineImpl, parent *mockStateProvider, mockCore *osmock.Core, expectedErr error) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Run(func(args mock.Arguments) {
					parent.UpdateState(StateComplete, pipeline)
					pipeline.SetSealed()
				}).Return(nil)
				mockCore.On("Persist").Return(expectedErr)
			},
			expectedErr:    errors.New("persist error"),
			expectedStates: []State{StateProcessing, StateWaitingPersist},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pipeline, mockCore, updateChan, parent := createPipeline(t)

			tc.setupMock(pipeline, parent, mockCore, tc.expectedErr)

			errChan := make(chan error)
			go func() {
				errChan <- pipeline.Run(context.Background(), mockCore, parent.GetState())
			}()

			// Send parent update to trigger processing
			parent.UpdateState(StateProcessing, pipeline)

			waitForStateUpdates(t, updateChan, errChan, tc.expectedStates...)

			waitForError(t, errChan, tc.expectedErr)
		})
	}
}

// TestSetSealed verifies that the pipeline correctly sets the sealed flag.
func TestSetSealed(t *testing.T) {
	pipeline, _, _, _ := createPipeline(t)

	pipeline.SetSealed()
	assert.True(t, pipeline.isSealed.Load())
}

// TestValidateTransition verifies that the pipeline correctly validates state transitions.
func TestValidateTransition(t *testing.T) {

	allStates := []State{StatePending, StateProcessing, StateWaitingPersist, StateComplete, StateAbandoned}

	// these are all of the valid transitions from a state to another state
	validTransitions := map[State]map[State]bool{
		StatePending:        {StateProcessing: true, StateAbandoned: true},
		StateProcessing:     {StateWaitingPersist: true, StateAbandoned: true},
		StateWaitingPersist: {StateComplete: true, StateAbandoned: true},
		StateComplete:       {},
		StateAbandoned:      {},
	}

	// iterate through all possible transitions, and validate that the valid transitions succeed, and the invalid transitions fail
	pipeline, _, _, _ := createPipeline(t)
	for _, currentState := range allStates {
		for _, newState := range allStates[1:] {
			if currentState == newState {
				continue // skip since higher level code will handle this
			}

			err := pipeline.validateTransition(currentState, newState)

			if validTransitions[currentState][newState] {
				assert.NoError(t, err)
				continue
			}

			assert.ErrorIs(t, err, ErrInvalidTransition)
		}
	}
}
