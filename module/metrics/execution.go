package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Execution Metrics
const (
	// executionBlockReceivedToExecuted is a duration metric
	// from a block being received by an execution node to being executed
	executionBlockReceivedToExecuted = "execution_block_received_to_executed"
)

var (
	executionGasUsedPerBlockGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "gas_used_per_block",
		Namespace: "execution",
		Help:      "the gas used per block",
	})
	executionStateReadsPerBlockGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "state_reads_per_block",
		Namespace: "execution",
		Help:      "count of state access/read operations performed per block",
	})
	executionStateStorageDiskTotalGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "execution_state_storage_disk_total",
		Namespace: "execution",
		Help:      "the execution state size on disk in bytes",
	})
	executionStorageStateCommitmentGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "storage_state_commitment",
		Namespace: "execution",
		Help:      "the storage size of a state commitment in bytes",
	})
)

// StartBlockReceivedToExecuted starts a span to trace the duration of a block
// from being received for execution to execution being finished
func (c *Collector) StartBlockReceivedToExecuted(blockID flow.Identifier) {
	c.tracer.StartSpan(blockID, executionBlockReceivedToExecuted).
		SetTag("block_id", blockID.String)
}

// FinishBlockReceivedToExecuted finishes a span to trace the duration of a block
// from being received for execution to execution being finished
func (c *Collector) FinishBlockReceivedToExecuted(blockID flow.Identifier) {
	c.tracer.FinishSpan(blockID, executionBlockReceivedToExecuted)
}

// ExecutionGasUsedPerBlock reports gas used per block
func (c *Collector) ExecutionGasUsedPerBlock(gas uint64) {
	executionGasUsedPerBlockGauge.Set(float64(gas))
}

// ExecutionStateReadsPerBlock reports number of state access/read operations per block
func (c *Collector) ExecutionStateReadsPerBlock(reads uint64) {
	executionStateReadsPerBlockGauge.Set(float64(reads))
}

// ExecutionStateStorageDiskTotal reports the total storage size of the execution state on disk in bytes
func (c *Collector) ExecutionStateStorageDiskTotal(bytes int64) {
	executionStateStorageDiskTotalGauge.Set(float64(bytes))
}

// ExecutionStorageStateCommitment reports the storage size of a state commitment
func (c *Collector) ExecutionStorageStateCommitment(bytes int64) {
	executionStorageStateCommitmentGauge.Set(float64(bytes))
}
