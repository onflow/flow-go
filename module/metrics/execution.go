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
	executionStorageDiskTotalGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "storage_disk_total",
		Namespace: "execution",
		Help:      "the execution storage size on disk in GiB",
	})
	executionStorageStateCommitmentGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "storage_state_commitment",
		Namespace: "execution",
		Help:      "the storage size of a state commitment in GiB",
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

// ExecutionStorageDiskTotal reports the total storage size on disk in GiB
func (c *Collector) ExecutionStorageDiskTotal(gib int64) {
	executionStorageDiskTotalGauge.Set(float64(gib))
}

// ExecutionStorageStateCommitment reports the storage size of a state commitment
func (c *Collector) ExecutionStorageStateCommitment(gib int64) {
	executionStorageStateCommitmentGauge.Set(float64(gib))
}
