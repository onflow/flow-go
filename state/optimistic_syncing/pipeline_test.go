package pipeline

import (
	"errors"
	"testing"

	"github.com/qmuntal/stateless"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestNewPipeline verifies that a new pipeline is created with the correct initial state
func TestNewPipeline(t *testing.T) {
	logger := zerolog.Nop()
	opMock := NewOperationMock()

	p := NewPipeline(
		logger,
		opMock.Download,
		opMock.Index,
		opMock.Persist,
	)

	// Verify initial state
	assert.Equal(t, StateReady, p.CurrentState())
	assert.False(t, p.IsDescendantOfSealed())
	assert.False(t, p.IsSealed())
}

// TestStateTransitions verifies that operations are called in the correct order
func TestStateTransitions(t *testing.T) {
	opMock := NewOperationMock()

	// Setup expectations - each operation will be called exactly once
	opMock.On("Download").Return(nil).Once()
	opMock.On("Index").Return(nil).Once()
	opMock.On("Persist").Return(nil).Once()

	// Track state changes
	stateChanges := make([]string, 0)

	// Create pipeline with mocked operations and state listener
	p := NewPipeline(
		zerolog.Nop(),
		opMock.Download,
		opMock.Index,
		opMock.Persist,
		WithStateListener(func(state string) {
			stateChanges = append(stateChanges, state)
		}),
	)

	p.UpdateFromParent(ParentUpdate{
		ParentState:        StateIndexing,
		DescendsFromSealed: true,
		IsSealed:           false,
	})

	// Verify state progression so far
	assert.Contains(t, stateChanges, StateDownloading)
	assert.Contains(t, stateChanges, StateIndexing)
	assert.Contains(t, stateChanges, StateWaitingPersist)

	// Verify operations were called correctly
	opMock.AssertNumberOfCalls(t, "Download", 1)
	opMock.AssertNumberOfCalls(t, "Index", 1)
	opMock.AssertNumberOfCalls(t, "Persist", 0)

	// Trigger persisting
	p.UpdateFromParent(ParentUpdate{
		ParentState:        StateComplete,
		DescendsFromSealed: true,
		IsSealed:           true,
	})

	// Verify complete state progression
	assert.Contains(t, stateChanges, StatePersisting)
	assert.Contains(t, stateChanges, StateComplete)

	// Verify persist was called
	opMock.AssertNumberOfCalls(t, "Persist", 1)

	// Verify all expectations were met
	opMock.AssertExpectations(t)
}

// TestPipelineCancellation verifies that the pipeline cancels when conditions change
func TestPipelineCancellation(t *testing.T) {
	opMock := NewOperationMock()

	// Setup expectations - each operation will be called exactly once
	opMock.On("Download").Return(nil).Once()
	opMock.On("Index").Return(nil).Once()
	opMock.On("Persist").Return(nil).Once()

	// Create pipeline with controlled download and state listener
	stateChanges := make([]string, 0)

	logger := zerolog.Nop()
	p := NewPipeline(
		logger,
		opMock.Download,
		opMock.Index,
		opMock.Persist,
		WithStateListener(func(state string) {
			stateChanges = append(stateChanges, state)
		}),
	)

	// Start the transition to downloading
	p.UpdateFromParent(ParentUpdate{
		ParentState:        StateIndexing,
		DescendsFromSealed: true,
		IsSealed:           false,
	})

	// Start the transition to downloading
	p.UpdateFromParent(ParentUpdate{
		ParentState:        StateIndexing,
		DescendsFromSealed: false,
		IsSealed:           false,
	})

	assert.Contains(t, stateChanges, StateCanceled)
}

// TestOperationFailures verifies that the pipeline handles operation failures correctly
func TestOperationFailures(t *testing.T) {
	tests := []struct {
		name          string
		downloadErr   error
		indexErr      error
		persistErr    error
		expectedState string
		setupPipeline func(*Pipeline)
	}{
		{
			name:          "download failure leads to canceled state",
			downloadErr:   errors.New("download error"),
			indexErr:      nil,
			persistErr:    nil,
			expectedState: StateCanceled,
			setupPipeline: func(p *Pipeline) {
				p.UpdateFromParent(ParentUpdate{
					ParentState:        StateIndexing,
					DescendsFromSealed: true,
					IsSealed:           false,
				})
			},
		},
		{
			name:          "index failure leads to canceled state",
			downloadErr:   nil,
			indexErr:      errors.New("index error"),
			persistErr:    nil,
			expectedState: StateCanceled,
			setupPipeline: func(p *Pipeline) {
				// Set state to downloading and manually fire download complete
				p.stateMachine = stateless.NewStateMachine(StateDownloading)
				p.pipelineInitialize()
				p.UpdateFromParent(ParentUpdate{
					ParentState:        StateIndexing,
					DescendsFromSealed: true,
					IsSealed:           false,
				})
				p.Fire(TriggerDownloadComplete)
			},
		},
		{
			name:          "persist failure leads to canceled state",
			downloadErr:   nil,
			indexErr:      nil,
			persistErr:    errors.New("persist error"),
			expectedState: StateCanceled,
			setupPipeline: func(p *Pipeline) {
				// Set state to waiting_persist and simulate conditions for persisting
				p.stateMachine = stateless.NewStateMachine(StateWaitingPersist)
				p.pipelineInitialize()
				p.UpdateFromParent(ParentUpdate{
					ParentState:        StateComplete,
					DescendsFromSealed: true,
					IsSealed:           true,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track final state
			var finalState string

			// Create mock
			opMock := NewOperationMock()

			// Configure mock behavior
			if tt.downloadErr != nil {
				opMock.On("Download").Return(tt.downloadErr)
			} else {
				opMock.On("Download").Return(nil)
			}

			if tt.indexErr != nil {
				opMock.On("Index").Return(tt.indexErr)
			} else {
				opMock.On("Index").Return(nil)
			}

			if tt.persistErr != nil {
				opMock.On("Persist").Return(tt.persistErr)
			} else {
				opMock.On("Persist").Return(nil)
			}

			// Create pipeline with mocked operations
			p := NewPipeline(
				zerolog.Nop(),
				opMock.Download,
				opMock.Index,
				opMock.Persist,
				WithStateListener(func(state string) {
					finalState = state
				}),
			)

			tt.setupPipeline(p)

			// Wait for the expected state
			assert.Equal(t, tt.expectedState, finalState)
		})
	}
}

// Mock for the operation functions
type OperationMock struct {
	mock.Mock
}

func NewOperationMock() *OperationMock {
	return &OperationMock{}
}

func (m *OperationMock) Download() error {
	args := m.Called()
	return args.Error(0)
}

func (m *OperationMock) Index() error {
	args := m.Called()
	return args.Error(0)
}

func (m *OperationMock) Persist() error {
	args := m.Called()
	return args.Error(0)
}
