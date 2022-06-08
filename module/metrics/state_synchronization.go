package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

type ExecutionDataServiceCollector struct {
	executionDataAddDuration prometheus.Histogram
	executionDataGetDuration prometheus.Histogram

	executionDataAddInProgress prometheus.Gauge
	executionDataGetInProgress prometheus.Gauge

	executionDataAddFailCount prometheus.Counter
	executionDataGetFailCount prometheus.Counter

	executionDataBlobTreeSize prometheus.Histogram
}

func NewExecutionDataServiceCollector() module.ExecutionDataServiceMetrics {
	executionDataAddDuration := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataService,
		Name:      "execution_data_add_duration_ms",
		Help:      "the duration of execution data add operation",
		Buckets:   []float64{1, 100, 500, 1000, 2000, 5000},
	})

	executionDataAddInProgress := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataService,
		Name:      "execution_data_add_in_progress",
		Help:      "number of concurrently running execution data add operations",
	})

	executionDataAddFailCount := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataService,
		Name:      "execution_data_add_fail_total",
		Help:      "number of failed execution data add operations",
	})

	executionDataGetDuration := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataService,
		Name:      "execution_data_get_duration_ms",
		Help:      "the duration of execution data get operation",
		Buckets:   []float64{1, 100, 500, 1000, 2000, 5000},
	})

	executionDataGetInProgress := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataService,
		Name:      "execution_data_get_in_progress",
		Help:      "number of concurrently running execution data get operations",
	})

	executionDataGetFailCount := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataService,
		Name:      "execution_data_get_fail_total",
		Help:      "number of failed execution data get operations",
	})

	executionDataBlobTreeSize := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataService,
		Name:      "execution_data_blob_tree_size",
		Help:      "the size execution data blob tree",
		Buckets:   []float64{1, 1000, 10000, 50000, 100000, 500000, 1000000, 10000000, 50000000, 100000000, 250000000, 500000000},
	})

	return &ExecutionDataServiceCollector{
		executionDataAddDuration:   executionDataAddDuration,
		executionDataAddInProgress: executionDataAddInProgress,
		executionDataAddFailCount:  executionDataAddFailCount,
		executionDataGetDuration:   executionDataGetDuration,
		executionDataGetInProgress: executionDataGetInProgress,
		executionDataGetFailCount:  executionDataGetFailCount,
		executionDataBlobTreeSize:  executionDataBlobTreeSize,
	}
}

func (ec *ExecutionDataServiceCollector) ExecutionDataAddStarted() {
	ec.executionDataAddInProgress.Inc()
}

func (ec *ExecutionDataServiceCollector) ExecutionDataAddFinished(duration time.Duration, success bool, blobTreeSize uint64) {
	ec.executionDataAddInProgress.Dec()
	ec.executionDataAddDuration.Observe(float64(duration.Milliseconds()))
	if !success {
		ec.executionDataAddFailCount.Inc()
	} else {
		ec.executionDataBlobTreeSize.Observe(float64(blobTreeSize))
	}
}

func (ec *ExecutionDataServiceCollector) ExecutionDataGetStarted() {
	ec.executionDataGetInProgress.Inc()
}

func (ec *ExecutionDataServiceCollector) ExecutionDataGetFinished(duration time.Duration, success bool, blobTreeSize uint64) {
	ec.executionDataGetInProgress.Dec()
	ec.executionDataGetDuration.Observe(float64(duration.Milliseconds()))
	if !success {
		ec.executionDataGetFailCount.Inc()
	} else {
		ec.executionDataBlobTreeSize.Observe(float64(blobTreeSize))
	}
}
