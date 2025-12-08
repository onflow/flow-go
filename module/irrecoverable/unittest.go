package irrecoverable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSignalerContext is a SignalerContext that can be used in tests to assert that an error is thrown.
// It embeds a [mock.Mock], so it can be used to assert that Throw is called with a specific error.
// Use NewMockSignalerContextExpectError to create a new MockSignalerContext that expects a specific error,
// otherwise NewMockSignalerContext.
type MockSignalerContext struct {
	context.Context
	*mock.Mock
}

var _ SignalerContext = &MockSignalerContext{}

func (m MockSignalerContext) sealed() {}

func (m MockSignalerContext) Throw(err error) {
	m.Called(err)
}

func NewMockSignalerContext(t *testing.T, ctx context.Context) *MockSignalerContext {
	m := &MockSignalerContext{
		Context: ctx,
		Mock:    &mock.Mock{},
	}
	m.Mock.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

// NewMockSignalerContextWithCancel creates a new MockSignalerContext with a cancel function.
func NewMockSignalerContextWithCancel(t *testing.T, parent context.Context) (*MockSignalerContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	return NewMockSignalerContext(t, ctx), cancel
}

// NewMockSignalerContextExpectError creates a new MockSignalerContext which expects a specific error to be thrown.
func NewMockSignalerContextExpectError(t *testing.T, ctx context.Context, err error) *MockSignalerContext {
	require.NotNil(t, err)
	m := NewMockSignalerContext(t, ctx)

	// since we expect an error, we should expect a call to Throw
	m.On("Throw", err).Once().Return()

	return m
}

// NewMockSignalerContextWithCallback creates a new MockSignalerContext that will call the provided
// callback if Throw is called, passing the error that is thrown.
// This can be used for performing special assertions on the errors, or adding hooks for test logic.
//
//	signalerCtx := irrecoverable.NewMockSignalerContextWithCallback(t, ctx, func(err error) {
//		cancel()
//		require.ErrorIs(t, err, expectedErr)
//	})
func NewMockSignalerContextWithCallback(t *testing.T, ctx context.Context, fn func(error)) *MockSignalerContext {
	require.NotNil(t, fn)
	m := NewMockSignalerContext(t, ctx)

	// since we expect an error, we should expect a call to Throw
	m.On("Throw", mock.Anything).Run(func(args mock.Arguments) {
		err := args[0].(error)
		fn(err)
	}).Maybe()

	return m
}
