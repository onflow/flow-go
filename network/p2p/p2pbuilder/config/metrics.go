package p2pconfig

import (
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
)

type MetricsConfig struct {
	HeroCacheFactory metrics.HeroCacheMetricsFactory
	Metrics          module.LibP2PMetrics
}
