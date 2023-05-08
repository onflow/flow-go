package p2pconfig

import (
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
)

// MetricsConfig is a wrapper around the metrics configuration for the libp2p node.
// It is used to pass the metrics configuration to the libp2p node builder.
type MetricsConfig struct {
	// HeroCacheFactory is the factory for the HeroCache metrics. It is used to
	// create a HeroCache metrics instance for each cache when needed. By passing
	// the factory to the libp2p node builder, the libp2p node can create the
	// HeroCache metrics instance for each cache internally, which reduces the
	// number of arguments needed to be passed to the libp2p node builder.
	HeroCacheFactory metrics.HeroCacheMetricsFactory

	// LibP2PMetrics is the metrics instance for the libp2p node.
	Metrics module.LibP2PMetrics
}
