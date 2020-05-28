package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/engine"
)

type EntriesFunc func() uint

type MempoolCollector struct {
	unit         *engine.Unit
	entries      *prometheus.GaugeVec
	interval     time.Duration
	delay        time.Duration
	entriesFuncs map[string]EntriesFunc // keeps map of registered EntriesFunc of mempools
}

func NewMempoolCollector(interval time.Duration) *MempoolCollector {

	mc := &MempoolCollector{
		unit:         engine.NewUnit(),
		interval:     interval,
		delay:        0,
		entriesFuncs: make(map[string]EntriesFunc),

		entries: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "entries_total",
			Namespace: namespaceStorage,
			Subsystem: subsystemMempool,
			Help:      "the number of entries in the mempool",
		}, []string{LabelResource}),
	}

	return mc
}

func (mc *MempoolCollector) MempoolEntries(resource string, entries uint) {
	mc.entries.With(prometheus.Labels{LabelResource: resource}).Set(float64(entries))
}

// Register registers entriesFunc for a resource
func (mc *MempoolCollector) Register(resource string, entriesFunc EntriesFunc) error {
	mc.unit.Lock()
	defer mc.unit.Unlock()

	if _, ok := mc.entriesFuncs[resource]; ok {
		return fmt.Errorf("cannot register resource, already exists: %s", resource)
	}

	mc.entriesFuncs[resource] = entriesFunc

	return nil
}

func (mc *MempoolCollector) Ready() <-chan struct{} {
	mc.unit.LaunchPeriodically(mc.gaugeEntries, mc.interval, mc.delay)
	return mc.unit.Ready()
}

func (mc *MempoolCollector) Done() <-chan struct{} {

	return mc.unit.Done()
}

// gaugeEntries iterates over the registered entries functions
// and calls MempoolEntries on them to capture the size of registered mempools
func (mc *MempoolCollector) gaugeEntries() {
	for r, f := range mc.entriesFuncs {
		mc.MempoolEntries(r, f())
	}
}
