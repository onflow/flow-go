package irrecoverable

import (
	"context"
	"testing"
)

type MockSignalerContext struct {
	context.Context
	t *testing.T
}

var _ SignalerContext = &MockSignalerContext{}

func (m MockSignalerContext) sealed() {}

func (m MockSignalerContext) Throw(err error) {
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
