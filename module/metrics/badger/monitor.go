package badger

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	badgermetrics "github.com/dgraph-io/badger/v2/y"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/module"
)

// The interval at which we measure and report metrics.
const checkInterval = 5 * time.Second

// Monitor implements passive monitoring of Badger database metrics.
type Monitor struct {
	unit    *engine.Unit
	metrics module.Metrics
	db      *badger.DB
}

// NewMonitor returns a new monitor for reporting Badger metrics.
func NewMonitor(metrics module.Metrics, db *badger.DB) *Monitor {
	monitor := &Monitor{
		unit:    engine.NewUnit(),
		metrics: metrics,
		db:      db,
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
			lsm, vlog := m.db.Size()
			m.metrics.BadgerLSMSize(lsm)
			m.metrics.BadgerVLogSize(vlog)

			// sample default badger metrics (from badger/v2/y/metrics.go)
			m.metrics.BadgerNumReads(badgermetrics.NumReads.Value())
			m.metrics.BadgerNumWrites(badgermetrics.NumWrites.Value())
			m.metrics.BadgerNumBytesRead(badgermetrics.NumBytesRead.Value())
			m.metrics.BadgerNumBytesWritten(badgermetrics.NumBytesWritten.Value())
			m.metrics.BadgerNumGets(badgermetrics.NumGets.Value())
			m.metrics.BadgerNumMemtableGets(badgermetrics.NumMemtableGets.Value())
		}
	}
}
