package executor

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAggregator(t *testing.T) {
	// Create mock executors
	mockExecutor1 := &MockExecutor{}
	mockExecutor2 := &MockExecutor{}
	mockExecutor3 := &MockExecutor{}

	// Create AggregatedExecutor
	aggregator := NewAggregatedExecutor([]IterationExecutor{mockExecutor1, mockExecutor2, mockExecutor3})

	// Test case 1: All executors succeed
	blockID := unittest.IdentifierFixture()
	batch := mock.NewReaderBatchWriter(t)

	err := aggregator.ExecuteByBlockID(blockID, batch)

	require.NoError(t, err)
	require.Equal(t, 1, mockExecutor1.CallCount)
	require.Equal(t, 1, mockExecutor2.CallCount)
	require.Equal(t, 1, mockExecutor3.CallCount)

	// Test case 2: Second executor fails
	mockExecutor1.Reset()
	mockExecutor2.Reset()
	mockExecutor3.Reset()
	mockExecutor2.ShouldFail = true

	err = aggregator.ExecuteByBlockID(blockID, batch)

	require.Error(t, err)
	require.Equal(t, 1, mockExecutor1.CallCount)
	require.Equal(t, 1, mockExecutor2.CallCount)
	require.Equal(t, 0, mockExecutor3.CallCount)
}

// MockExecutor is a mock implementation of IterationExecutor
type MockExecutor struct {
	CallCount  int
	ShouldFail bool
}

var _ IterationExecutor = (*MockExecutor)(nil)

func (m *MockExecutor) ExecuteByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) error {
	m.CallCount++
	if m.ShouldFail {
		return fmt.Errorf("mock error")
	}
	return nil
}

func (m *MockExecutor) Reset() {
	m.CallCount = 0
	m.ShouldFail = false
}
