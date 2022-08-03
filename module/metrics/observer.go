package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ObserverCollector struct {
	rpcs *prometheus.CounterVec
}

func NewObserverCollector() *ObserverCollector {
	return &ObserverCollector{
		rpcs: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceObserver,
			Subsystem: subsystemRouter,
			Name:      "rpc_counter",
			Help:      "tracking error/success rate of each rpc",
		}, []string{"handler", "rpc", "error"}),
	}
}

func (oc *ObserverCollector) RecordRPC(handler, rpc string, err bool) {
	oc.rpcs.With(prometheus.Labels{
		"handler": handler,
		"rpc":     rpc,
		"error":   strconv.FormatBool(err),
	}).Inc()
}
