package irrecoverable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockSignalerContext is a SignalerContext which will immediately fail a test if an error is thrown.
type MockSignalerContext struct {
	context.Context
	t           *testing.T
	expectError error
}

var _ SignalerContext = &MockSignalerContext{}

func (m MockSignalerContext) sealed() {}

func (m MockSignalerContext) Throw(err error) {
	if m.expectError != nil {
		assert.EqualError(m.t, err, m.expectError.Error())
		return
	}
	m.t.Fatalf("mock signaler context received error: %v", err)
}

func NewMockSignalerContext(t *testing.T, ctx context.Context) *MockSignalerContext {
	return &MockSignalerContext{
		Context: ctx,
		t:       t,
	}
}

func NewMockSignalerContextWithCancel(t *testing.T, parent context.Context) (*MockSignalerContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	return NewMockSignalerContext(t, ctx), cancel
}

func NewMockSignalerContextExpectError(t *testing.T, ctx context.Context, err error) *MockSignalerContext {
	return &MockSignalerContext{
		Context:     ctx,
		t:           t,
		expectError: err,
	}
}
