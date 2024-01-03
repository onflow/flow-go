package storehouse

import (
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/module"
)

type RegisterStoreMetrics struct {
	collector module.ExecutionMetrics
}

var _ execution.RegisterStoreNotifier = (*RegisterStoreMetrics)(nil)

func NewRegisterStoreMetrics(collector module.ExecutionMetrics) *RegisterStoreMetrics {
	return &RegisterStoreMetrics{
		collector: collector,
	}
}

func (m *RegisterStoreMetrics) OnFinalizedAndExecutedHeightUpdated(height uint64) {
	m.collector.ExecutionLastFinalizedExecutedBlockHeight(height)
}
