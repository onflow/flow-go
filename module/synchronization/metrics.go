package synchronization

import (
	"github.com/onflow/flow-go/model/flow"
)

type SynchronizationMetrics interface {
	// record pruned blocks. requested and received times might be zero values
	PrunedBlockById(status *Status)

	PrunedBlockByHeight(status *Status)

	// totalByHeight and totalById are the number of blocks pruned for blocks requested by height and by id
	// storedByHeight and storedById are the number of blocks still stored by height and id
	PrunedBlocks(totalByHeight, totalById, storedByHeight, storedById int)

	RangeRequested(ran flow.Range)

	BatchRequested(batch flow.Batch)
}

type NoopMetrics struct{}

func (nc *NoopMetrics) PrunedBlockById(status *Status)                                        {}
func (nc *NoopMetrics) PrunedBlockByHeight(status *Status)                                    {}
func (nc *NoopMetrics) PrunedBlocks(totalByHeight, totalById, storedByHeight, storedById int) {}
func (nc *NoopMetrics) RangeRequested(ran flow.Range)                                         {}
func (nc *NoopMetrics) BatchRequested(batch flow.Batch)                                       {}

type MetricsCollector struct {
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

func (s *MetricsCollector) PrunedBlockById(status *Status) {

}

func (s *MetricsCollector) PrunedBlockByHeight(status *Status) {

}

func (s *MetricsCollector) PrunedBlocks(totalByHeight, totalById, storedByHeight, storedById int) {

}

func (s *MetricsCollector) RangeRequested(ran flow.Range) {

}

func (s *MetricsCollector) BatchRequested(batch flow.Batch) {

}
