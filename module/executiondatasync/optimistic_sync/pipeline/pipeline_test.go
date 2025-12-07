package pipeline

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	osmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestPipelineStateTransitions verifies that the pipeline correctly transitions
// through states when provided with the correct conditions.
func TestPipelineStateTransitions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pipeline, mockCore, updateChan, parent := createPipeline(t)

		pipeline.SetSealed()
		parent.UpdateState(optimistic_sync.StateComplete, pipeline)

		mockCore.On("Download", mock.Anything).Return(nil)
		mockCore.On("Index").Return(nil)
		mockCore.On("Persist").Return(nil)

		assert.Equal(t, optimistic_sync.StatePending, pipeline.GetState(), "Pipeline should start in Pending state")

		go func() {
			err := pipeline.Run(context.Background(), mockCore, parent.GetState())
			require.NoError(t, err)
		}()

		for _, expected := range []optimistic_sync.State{optimistic_sync.StateProcessing, optimistic_sync.StateWaitingPersist, optimistic_sync.StateComplete} {
			synctest.Wait()
			assertUpdate(t, updateChan, expected)
		}

		// wait for Run goroutine to finish
		synctest.Wait()
		assertNoUpdate(t, pipeline, updateChan, optimistic_sync.StateComplete)
	})
}

// TestPipelineParentDependentTransitions verifies that a pipeline's transitions
// depend on the parent pipeline's state.
func TestPipelineParentDependentTransitions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pipeline, mockCore, updateChan, parent := createPipeline(t)

		assert.Equal(t, optimistic_sync.StatePending, pipeline.GetState(), "Pipeline should start in Pending state")

		go func() {
			err := pipeline.Run(context.Background(), mockCore, parent.GetState())
			require.NoError(t, err)
		}()

		// 1. Initial update - parent in Ready state
		parent.UpdateState(optimistic_sync.StatePending, pipeline)

		// Check that pipeline remains in Ready state
		synctest.Wait()
		assertNoUpdate(t, pipeline, updateChan, optimistic_sync.StatePending)

		// 2. Update parent to downloading
		parent.UpdateState(optimistic_sync.StateProcessing, pipeline)

		// Pipeline should now call Download and Index within the processing state, then progress to
		// WaitingPersist and stop
		mockCore.On("Download", mock.Anything).Return(nil)
		mockCore.On("Index").Return(nil)
		for _, expected := range []optimistic_sync.State{optimistic_sync.StateProcessing, optimistic_sync.StateWaitingPersist} {
			synctest.Wait()
			assertUpdate(t, updateChan, expected)
		}
		assert.Equal(t, optimistic_sync.StateWaitingPersist, pipeline.GetState(), "Pipeline should be in StateWaitingPersist state")

		// pipeline should remain in WaitingPersist state
		synctest.Wait()
		assertNoUpdate(t, pipeline, updateChan, optimistic_sync.StateWaitingPersist)

		// 3. Update parent to complete - should allow persisting when sealed
		parent.UpdateState(optimistic_sync.StateComplete, pipeline)

		// this alone should not allow the pipeline to progress to any other state
		synctest.Wait()
		assertNoUpdate(t, pipeline, updateChan, optimistic_sync.StateWaitingPersist)

		// 4. Mark the execution result as sealed, this should allow the pipeline to progress to Complete state
		pipeline.SetSealed()
		mockCore.On("Persist").Return(nil)

		// Wait for pipeline to complete
		synctest.Wait()
		assertUpdate(t, updateChan, optimistic_sync.StateComplete)

		// Run should complete without error
		synctest.Wait()
		assertNoUpdate(t, pipeline, updateChan, optimistic_sync.StateComplete)
	})
}

// TestParentAbandoned verifies that a pipeline is properly abandoned when
// the parent pipeline is abandoned.
func TestAbandoned(t *testing.T) {
	t.Run("starts already abandoned", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			pipeline, mockCore, updateChan, parent := createPipeline(t)

			mockCore.On("Abandon").Return(nil)

			pipeline.Abandon()

			go func() {
				err := pipeline.Run(context.Background(), mockCore, parent.GetState())
				require.NoError(t, err)
			}()

			// first state must be abandoned
			synctest.Wait()
			assertUpdate(t, updateChan, optimistic_sync.StateAbandoned)

			// wait for Run goroutine to finish
			synctest.Wait()
			assertNoUpdate(t, pipeline, updateChan, optimistic_sync.StateAbandoned)
		})
	})

	// Test cases abandoning during different stages of processing
	testCases := []struct {
		name           string
		setupMock      func(*Pipeline, *mockStateProvider, *osmock.Core)
		onStateFns     map[optimistic_sync.State]func(*Pipeline, *mockStateProvider)
		expectedStates []optimistic_sync.State
	}{
		{
			name: "Abandon during download",
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.
					On("Download", mock.Anything).
					Return(func(ctx context.Context) error {
						pipeline.Abandon()
						<-ctx.Done() // abandon should cause context to be canceled
						return ctx.Err()
					})
			},
			expectedStates: []optimistic_sync.State{optimistic_sync.StateProcessing, optimistic_sync.StateAbandoned},
		},
		{
			name: "Parent abandoned during download",
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.
					On("Download", mock.Anything).
					Return(func(ctx context.Context) error {
						parent.UpdateState(optimistic_sync.StateAbandoned, pipeline)
						<-ctx.Done() // abandon should cause context to be canceled
						return ctx.Err()
					})
			},
			expectedStates: []optimistic_sync.State{optimistic_sync.StateProcessing, optimistic_sync.StateAbandoned},
		},
		{
			name: "Abandon during index",
			// Note: indexing will complete, and the pipeline will transition to waiting persist
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Run(func(args mock.Arguments) {
					pipeline.Abandon()
				}).Return(nil)
			},
			expectedStates: []optimistic_sync.State{optimistic_sync.StateProcessing, optimistic_sync.StateAbandoned},
		},
		{
			name: "Parent abandoned during index",
			// Note: indexing will complete, and the pipeline will transition to waiting persist
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Run(func(args mock.Arguments) {
					parent.UpdateState(optimistic_sync.StateAbandoned, pipeline)
				}).Return(nil)
			},
			expectedStates: []optimistic_sync.State{optimistic_sync.StateProcessing, optimistic_sync.StateAbandoned},
		},
		{
			name: "Abandon during waiting to persist",
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Return(nil)
			},
			onStateFns: map[optimistic_sync.State]func(*Pipeline, *mockStateProvider){
				optimistic_sync.StateWaitingPersist: func(pipeline *Pipeline, parent *mockStateProvider) {
					pipeline.Abandon()
				},
			},
			expectedStates: []optimistic_sync.State{optimistic_sync.StateProcessing, optimistic_sync.StateWaitingPersist, optimistic_sync.StateAbandoned},
		},
		{
			name: "Parent abandoned during waiting to persist",
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Return(nil)
			},
			onStateFns: map[optimistic_sync.State]func(*Pipeline, *mockStateProvider){
				optimistic_sync.StateWaitingPersist: func(pipeline *Pipeline, parent *mockStateProvider) {
					parent.UpdateState(optimistic_sync.StateAbandoned, pipeline)
				},
			},
			expectedStates: []optimistic_sync.State{optimistic_sync.StateProcessing, optimistic_sync.StateWaitingPersist, optimistic_sync.StateAbandoned},
		},
		// Note: it does not make sense to abandon during persist, since it will only be run when:
		// 1. the parent is already complete
		// 2. the pipeline's result is sealed
		// At that point, there are no conditions that would cause the pipeline to transition to any other state
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				pipeline, mockCore, updateChan, parent := createPipeline(t)
				tc.setupMock(pipeline, parent, mockCore)

				mockCore.On("Abandon").Return(nil)

				go func() {
					err := pipeline.Run(context.Background(), mockCore, parent.GetState())
					require.NoError(t, err)
				}()

				// Send parent update to start processing
				parent.UpdateState(optimistic_sync.StateProcessing, pipeline)

				for _, expected := range tc.expectedStates {
					synctest.Wait()
					assertUpdate(t, updateChan, expected)

					if fn, ok := tc.onStateFns[expected]; ok {
						fn(pipeline, parent)
					}
				}

				// wait for Run goroutine to finish
				synctest.Wait()
				assertNoUpdate(t, pipeline, updateChan, optimistic_sync.StateAbandoned)
			})
		})
	}
}

// TestPipelineContextCancellation tests the Run method's context cancelation behavior during different stages of processing
func TestPipelineContextCancellation(t *testing.T) {
	// Test cases for different stages of processing
	testCases := []struct {
		name      string
		setupMock func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) context.Context
	}{
		{
			name: "Cancel before download starts",
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				// no Core methods called
				return ctx
			},
		},
		{
			name: "Cancel during download",
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) context.Context {
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
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core) context.Context {
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
			synctest.Test(t, func(t *testing.T) {
				pipeline, mockCore, _, parent := createPipeline(t)

				parent.UpdateState(optimistic_sync.StateComplete, pipeline)
				pipeline.SetSealed()

				ctx := tc.setupMock(pipeline, parent, mockCore)

				go func() {
					err := pipeline.Run(ctx, mockCore, parent.GetState())
					require.ErrorIs(t, err, context.Canceled)
				}()

				// wait for Run goroutine to finish
				synctest.Wait()
			})
		})
	}
}

// TestPipelineErrorHandling verifies that errors from Core methods are properly
// propagated back to the caller.
func TestPipelineErrorHandling(t *testing.T) {
	// Test cases for different stages of processing
	testCases := []struct {
		name           string
		setupMock      func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core, expectedErr error)
		expectedErr    error
		expectedStates []optimistic_sync.State
	}{
		{
			name: "Download Error",
			setupMock: func(pipeline *Pipeline, _ *mockStateProvider, mockCore *osmock.Core, expectedErr error) {
				mockCore.On("Download", mock.Anything).Return(expectedErr)
			},
			expectedErr:    errors.New("download error"),
			expectedStates: []optimistic_sync.State{optimistic_sync.StateProcessing},
		},
		{
			name: "Index Error",
			setupMock: func(pipeline *Pipeline, _ *mockStateProvider, mockCore *osmock.Core, expectedErr error) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Return(expectedErr)
			},
			expectedErr:    errors.New("index error"),
			expectedStates: []optimistic_sync.State{optimistic_sync.StateProcessing},
		},
		{
			name: "Persist Error",
			setupMock: func(pipeline *Pipeline, parent *mockStateProvider, mockCore *osmock.Core, expectedErr error) {
				mockCore.On("Download", mock.Anything).Return(nil)
				mockCore.On("Index").Run(func(args mock.Arguments) {
					parent.UpdateState(optimistic_sync.StateComplete, pipeline)
					pipeline.SetSealed()
				}).Return(nil)
				mockCore.On("Persist").Return(expectedErr)
			},
			expectedErr:    errors.New("persist error"),
			expectedStates: []optimistic_sync.State{optimistic_sync.StateProcessing, optimistic_sync.StateWaitingPersist},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				pipeline, mockCore, updateChan, parent := createPipeline(t)

				tc.setupMock(pipeline, parent, mockCore, tc.expectedErr)

				go func() {
					err := pipeline.Run(context.Background(), mockCore, parent.GetState())
					require.ErrorIs(t, err, tc.expectedErr)
				}()

				// Send parent update to trigger processing
				parent.UpdateState(optimistic_sync.StateProcessing, pipeline)

				for _, expected := range tc.expectedStates {
					synctest.Wait()
					assertUpdate(t, updateChan, expected)
				}

				// wait for Run goroutine to finish
				synctest.Wait()
				finalState := tc.expectedStates[len(tc.expectedStates)-1]
				assertNoUpdate(t, pipeline, updateChan, finalState)
			})
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

	allStates := []optimistic_sync.State{optimistic_sync.StatePending, optimistic_sync.StateProcessing, optimistic_sync.StateWaitingPersist, optimistic_sync.StateComplete, optimistic_sync.StateAbandoned}

	// these are all of the valid transitions from a state to another state
	validTransitions := map[optimistic_sync.State]map[optimistic_sync.State]bool{
		optimistic_sync.StatePending:        {optimistic_sync.StateProcessing: true, optimistic_sync.StateAbandoned: true},
		optimistic_sync.StateProcessing:     {optimistic_sync.StateWaitingPersist: true, optimistic_sync.StateAbandoned: true},
		optimistic_sync.StateWaitingPersist: {optimistic_sync.StateComplete: true, optimistic_sync.StateAbandoned: true},
		optimistic_sync.StateComplete:       {},
		optimistic_sync.StateAbandoned:      {},
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

			assert.ErrorIs(t, err, optimistic_sync.ErrInvalidTransition)
		}
	}
}

func assertNoUpdate(t *testing.T, pipeline *Pipeline, updateChan <-chan optimistic_sync.State, existingState optimistic_sync.State) {
	select {
	case update := <-updateChan:
		t.Errorf("Pipeline should remain in %s state, but transitioned to %s", existingState, update)
	default:
		assert.Equal(t, existingState, pipeline.GetState(), "Pipeline should remain in %s state", existingState)
	}
}

func assertUpdate(t *testing.T, updateChan <-chan optimistic_sync.State, expected optimistic_sync.State) {
	select {
	case update := <-updateChan:
		assert.Equal(t, expected, update, "Pipeline should transition to %s state", expected)
	default:
		t.Errorf("Pipeline should have transitioned to %s state", expected)
	}
}
