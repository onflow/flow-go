package optimistic_sync

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	osmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// mockStateProvider is a mock implementation of a parent state provider.
// It tracks the current state and notifies the pipeline when the state changes.
type mockStateProvider struct {
	state State
}

var _ PipelineStateProvider = (*mockStateProvider)(nil)

// NewMockStateProvider initializes a mockStateProvider with the default state StatePending.
func NewMockStateProvider() *mockStateProvider {
	return &mockStateProvider{
		state: StatePending,
	}
}

// UpdateState sets the internal state and triggers a pipeline update.
func (m *mockStateProvider) UpdateState(state State, pipeline *PipelineImpl) {
	m.state = state
	pipeline.OnParentStateUpdated(state)
}

// GetState returns the current internal state.

func (m *mockStateProvider) GetState() State {
	return m.state
}

// mockStateConsumer is a mock implementation used in tests to receive state updates from the pipeline.
// It exposes a buffered channel to capture the state transitions.
type mockStateConsumer struct {
	updateChan chan State
}

var _ PipelineStateConsumer = (*mockStateConsumer)(nil)

// NewMockStateConsumer creates a new instance of mockStateConsumer with a buffered channel.
func NewMockStateConsumer() *mockStateConsumer {
	return &mockStateConsumer{
		updateChan: make(chan State, 10),
	}
}

func (m *mockStateConsumer) OnStateUpdated(state State) {
	m.updateChan <- state
}

// waitForStateUpdates waits for a sequence of state updates to occur or timeout after 500ms.
// updates must be received in the correct order or the test will fail.
func waitForStateUpdates(t *testing.T, updateChan <-chan State, errChan <-chan error, expectedStates ...State) {
	done := make(chan struct{})
	unittest.RequireReturnsBefore(t, func() {
		for _, expected := range expectedStates {
			select {
			case <-done:
				return
			case err := <-errChan:
				require.NoError(t, err, "pipeline returned error")
			case update := <-updateChan:
				assert.Equalf(t, expected, update, "expected pipeline to transition to %s, but got %s", expected, update)
			}
		}
	}, 500*time.Millisecond, "Timeout waiting for state update")
	close(done) // make sure function exists after timeout
}

// waitForErrorWithCustomCheckers waits for an error from the errChan within 500ms
// and applies custom checker functions to validate the error.
// If no checkers are provided, it asserts that no error occurred.
func waitForErrorWithCustomCheckers(t *testing.T, errChan <-chan error, errorCheckers ...func(err error)) {
	unittest.RequireReturnsBefore(t, func() {
		err := <-errChan
		if len(errorCheckers) == 0 {
			assert.NoError(t, err, "Pipeline should complete without errors")
		} else {
			for _, checker := range errorCheckers {
				checker(err)
			}
		}
	}, 500*time.Millisecond, "Timeout waiting for error")
}

// waitForError waits for an error from the errChan within 500ms and asserts it matches the expected error.
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

// createPipeline initializes and returns a pipeline instance with its mock dependencies.
// It returns the pipeline, the mocked core, a state update channel, and the parent state provider.
func createPipeline(t *testing.T) (*PipelineImpl, *osmock.Core, <-chan State, *mockStateProvider) {
	mockCore := osmock.NewCore(t)
	parent := NewMockStateProvider()
	stateReceiver := NewMockStateConsumer()

	pipeline := NewPipeline(zerolog.Nop(), unittest.ExecutionResultFixture(), false, stateReceiver)

	return pipeline, mockCore, stateReceiver.updateChan, parent
}

// synctestWaitForStateUpdates waits for a sequence of state updates to occur using synctest.Wait.
// updates must be received in the correct order or the test will fail.
// TODO: refactor all tests to use the synctest approach.
func synctestWaitForStateUpdates(t *testing.T, updateChan <-chan State, expectedStates ...State) {
	for _, expected := range expectedStates {
		synctest.Wait()
		update, ok := <-updateChan
		require.True(t, ok, "update channel closed unexpectedly")
		assert.Equalf(t, expected, update, "expected pipeline to transition to %s, but got %s", expected, update)
	}
}

// synctestWaitForError waits for an error from the errChan using synctest.Wait and asserts it matches the expected error.
// TODO: refactor all tests to use the synctest approach.
func synctestWaitForError(t *testing.T, errChan <-chan error, expectedErr error) {
	synctest.Wait()
	err := <-errChan
	if expectedErr == nil {
		assert.NoError(t, err, "Pipeline should complete without errors")
	} else {
		assert.ErrorIs(t, err, expectedErr)
	}

}
