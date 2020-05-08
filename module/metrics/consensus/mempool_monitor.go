package consensus

import (
	"time"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
)

// The interval at which we measure and report metrics.
const checkInterval = 5 * time.Second

// Monitor implements passive monitoring of mempool metrics.
type Monitor struct {
	unit       *engine.Unit
	metrics    module.Metrics
	guarantees mempool.Guarantees
	receipts   mempool.Receipts
	approvals  mempool.Approvals
	seals      mempool.Seals
}

// NewMonitor returns a new monitor for reporting mempool metrics.
func NewMonitor(metrics module.Metrics, guarantees mempool.Guarantees, receipts mempool.Receipts,
	approvals mempool.Approvals, seals mempool.Seals) *Monitor {
	monitor := &Monitor{
		unit:       engine.NewUnit(),
		metrics:    metrics,
		guarantees: guarantees,
		receipts:   receipts,
		approvals:  approvals,
		seals:      seals,
	}
	return monitor
}

func (m *Monitor) Ready() <-chan struct{} {
	m.unit.Launch(m.monitor)
	return m.unit.Ready()
}

func (m *Monitor) Done() <-chan struct{} {
	return m.unit.Done()
}

// monitor reports metrics at each interval until it receives the done signal.
func (m *Monitor) monitor() {

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.unit.Quit():
			return
		case <-ticker.C:
			m.metrics.MempoolApprovalsSize(m.approvals.Size())
			m.metrics.MempoolGuaranteesSize(m.guarantees.Size())
			m.metrics.MempoolReceiptsSize(m.receipts.Size())
			m.metrics.MempoolSealsSize(m.seals.Size())
		}
	}
}
