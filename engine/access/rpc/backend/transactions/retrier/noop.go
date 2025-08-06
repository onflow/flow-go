package retrier

import (
	"github.com/onflow/flow-go/model/flow"
)

type NoopRetrier struct{}

var _ Retrier = (*NoopRetrier)(nil)

func NewNoopRetrier() *NoopRetrier {
	return &NoopRetrier{}
}

func (n *NoopRetrier) Retry(_ uint64) error {
	return nil
}

func (n *NoopRetrier) RegisterTransaction(_ uint64, _ *flow.TransactionBody) {}
