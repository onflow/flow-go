package networkmetrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/metrics/internal"
	"github.com/onflow/flow-go/network"
)

func NetworkReceiveCacheMetricsFactory(f metrics.HeroCacheMetricsFactory, networkType network.NetworkingType) module.HeroCacheMetrics {
	r := metrics.ResourceNetworkingReceiveCache
	if networkType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}
	return f(internal.NamespaceNetwork, r)
}

// DisallowListCacheMetricsFactory is the factory method for creating a new HeroCacheCollector for the disallow list cache.
// The disallow-list cache is used to keep track of peers that are disallow-listed and the reasons for it.
// Args:
// - f: the HeroCacheMetricsFactory to create the collector
// - networkingType: the networking type of the cache, i.e., whether it is used for the public or the private network
// Returns:
// - a HeroCacheMetrics for the disallow list cache
func DisallowListCacheMetricsFactory(f metrics.HeroCacheMetricsFactory, networkingType network.NetworkingType) module.HeroCacheMetrics {
	r := metrics.ResourceNetworkingDisallowListCache
	if networkingType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}
	return f(internal.NamespaceNetwork, r)
}

func NetworkDnsTxtCacheMetricsFactory(registrar prometheus.Registerer) *metrics.HeroCacheCollector {
	return metrics.NewHeroCacheCollector(internal.NamespaceNetwork, metrics.ResourceNetworkingDnsTxtCache, registrar)
}

func NetworkDnsIpCacheMetricsFactory(registrar prometheus.Registerer) *metrics.HeroCacheCollector {
	return metrics.NewHeroCacheCollector(internal.NamespaceNetwork, metrics.ResourceNetworkingDnsIpCache, registrar)
}

func DisallowListNotificationQueueMetricFactory(registrar prometheus.Registerer) *metrics.HeroCacheCollector {
	return metrics.NewHeroCacheCollector(internal.NamespaceNetwork, metrics.ResourceNetworkingDisallowListNotificationQueue, registrar)
}

func ApplicationLayerSpamRecordCacheMetricFactory(f metrics.HeroCacheMetricsFactory, networkType network.NetworkingType) module.HeroCacheMetrics {
	r := metrics.ResourceNetworkingApplicationLayerSpamRecordCache
	if networkType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}

	return f(internal.NamespaceNetwork, r)
}

func ApplicationLayerSpamRecordQueueMetricsFactory(f metrics.HeroCacheMetricsFactory, networkType network.NetworkingType) module.HeroCacheMetrics {
	r := metrics.ResourceNetworkingApplicationLayerSpamReportQueue
	if networkType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}
	return f(internal.NamespaceNetwork, r)
}

func GossipSubRPCMetricsObserverInspectorQueueMetricFactory(f metrics.HeroCacheMetricsFactory, networkType network.NetworkingType) module.HeroCacheMetrics {
	// we don't use the public prefix for the metrics here for sake of backward compatibility of metric name.
	r := metrics.ResourceNetworkingRpcMetricsObserverInspectorQueue
	if networkType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}
	return f(internal.NamespaceNetwork, r)
}

func GossipSubRPCInspectorQueueMetricFactory(f metrics.HeroCacheMetricsFactory, networkType network.NetworkingType) module.HeroCacheMetrics {
	// we don't use the public prefix for the metrics here for sake of backward compatibility of metric name.
	r := metrics.ResourceNetworkingRpcValidationInspectorQueue
	if networkType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}
	return f(internal.NamespaceNetwork, r)
}

func GossipSubRPCSentTrackerMetricFactory(f metrics.HeroCacheMetricsFactory, networkType network.NetworkingType) module.HeroCacheMetrics {
	// we don't use the public prefix for the metrics here for sake of backward compatibility of metric name.
	r := metrics.ResourceNetworkingRPCSentTrackerCache
	if networkType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}
	return f(internal.NamespaceNetwork, r)
}

func GossipSubRPCSentTrackerQueueMetricFactory(f metrics.HeroCacheMetricsFactory, networkType network.NetworkingType) module.HeroCacheMetrics {
	// we don't use the public prefix for the metrics here for sake of backward compatibility of metric name.
	r := metrics.ResourceNetworkingRPCSentTrackerQueue
	if networkType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}
	return f(internal.NamespaceNetwork, r)
}

func RpcInspectorNotificationQueueMetricFactory(f metrics.HeroCacheMetricsFactory, networkType network.NetworkingType) module.HeroCacheMetrics {
	r := metrics.ResourceNetworkingRpcInspectorNotificationQueue
	if networkType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}
	return f(internal.NamespaceNetwork, r)
}

func GossipSubRPCInspectorClusterPrefixedCacheMetricFactory(f metrics.HeroCacheMetricsFactory, networkType network.NetworkingType) module.HeroCacheMetrics {
	// we don't use the public prefix for the metrics here for sake of backward compatibility of metric name.
	r := metrics.ResourceNetworkingRpcClusterPrefixReceivedCache
	if networkType == network.PublicNetwork {
		r = PrependPublicPrefix(r)
	}
	return f(internal.NamespaceNetwork, r)
}

func FollowerCacheMetrics(registrar prometheus.Registerer) *metrics.HeroCacheCollector {
	return metrics.NewHeroCacheCollector(internal.NamespaceFollowerEngine, metrics.ResourceFollowerPendingBlocksCache, registrar)
}

// PrependPublicPrefix prepends the string "public" to the given string.
// This is used to distinguish between public and private metrics.
// Args:
// - str: the string to prepend, example: "my_metric"
// Returns:
// - the prepended string, example: "public_my_metric"
func PrependPublicPrefix(str string) string {
	return fmt.Sprintf("%s_%s", "public", str)
}
