package merr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
)

// mockCloser is a mock implementation of io.Closer
type mockCloser struct {
	closeError error
}

// Close is a mock implementation of io.Closer.Close
func (c *mockCloser) Close() error {
	return c.closeError
}

func TestCloseAndMergeError(t *testing.T) {
	// Create a mock closer
	closer := &mockCloser{}

	// Test case 1: no error
	err := CloseAndMergeError(closer, nil)
	require.Nil(t, err)

	// Test case 2: only original error
	err = CloseAndMergeError(closer, errors.New("original error"))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "original error")

	// Test case 3: only close error
	closer.closeError = errors.New("close error")
	err = CloseAndMergeError(closer, nil)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "close error")

	// Test case 4: both original error and close error
	err = CloseAndMergeError(closer, errors.New("original error"))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "original error")
	require.Contains(t, err.Error(), "close error")

	// Test case 5: original error is storage.ErrNotFound
	err = CloseAndMergeError(closer, fmt.Errorf("not found error: %w", storage.ErrNotFound))
	require.NotNil(t, err)
	require.True(t, errors.Is(err, storage.ErrNotFound))

	// Test case 6: close error is storage.ErrNotFound
	closer.closeError = fmt.Errorf("not found error: %w", storage.ErrNotFound)
	err = CloseAndMergeError(closer, nil)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, storage.ErrNotFound))

	// Test case 7: error check works with multierror
	closer.closeError = fmt.Errorf("exception")
	err = CloseAndMergeError(closer, fmt.Errorf("not found error: %w", storage.ErrNotFound))
	require.NotNil(t, err)
	require.True(t, errors.Is(err, storage.ErrNotFound))
}
