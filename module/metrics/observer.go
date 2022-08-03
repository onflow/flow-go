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
		}, []string{"rpc", "handler", "error", "fallback"}),
	}
}

func (oc *ObserverCollector) RecordRPC(handler, rpc string, err, fallback bool) {
	oc.rpcs.With(prometheus.Labels{
		"rpc":      rpc,
		"handler":  handler,
		"error":    strconv.FormatBool(err),
		"fallback": strconv.FormatBool(err),
	}).Inc()
}
