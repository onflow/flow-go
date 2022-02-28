package unittest

import (
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
)

func NoopHeroCacheMetricsRegistrationFunc(_ uint64) module.HeroCacheMetrics {
	return metrics.NewNoopCollector()
}
