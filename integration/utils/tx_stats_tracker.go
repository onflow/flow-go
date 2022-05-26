package utils

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/atomic"

	"github.com/jedib0t/go-pretty/table"
	"github.com/openhistogram/circonusllhist"
)

// TxStats holds stats about execution of a transaction
type TxStats struct {
	// TTF is transaction time to finality
	TTF time.Duration
	// TTE is transaction time to execution
	TTE time.Duration
	// TTS is transaction time to seal
	TTS time.Duration

	isExpired bool
}

// TxStatsTracker keep track of TxStats
type TxStatsTracker struct {
	TTF *circonusllhist.Histogram
	TTE *circonusllhist.Histogram
	TTS *circonusllhist.Histogram

	failures *atomic.Uint64
	count    *atomic.Uint64
}

// NewTxStatsTracker returns a new instance of StatsTracker
func NewTxStatsTracker() *TxStatsTracker {
	return &TxStatsTracker{
		TTF: circonusllhist.New(),
		TTE: circonusllhist.New(),
		TTS: circonusllhist.New(),

		failures: atomic.NewUint64(0),
		count:    atomic.NewUint64(0),
	}
}

// AddTxStats adds a new TxStats to the tracker
func (st *TxStatsTracker) AddTxStats(tt *TxStats) {
	st.count.Inc()

	if tt.isExpired {
		st.failures.Inc()
		return
	}

	if tt.TTF > 0 {
		st.TTF.RecordDuration(tt.TTF)
	}
	if tt.TTE > 0 {
		st.TTE.RecordDuration(tt.TTE)
	}
	if tt.TTS > 0 {
		st.TTS.RecordDuration(tt.TTS)
	}
}

// TotalTxSubmited returns the total transaction submited
func (st *TxStatsTracker) TotalTxSubmited() uint64 {
	return st.count.Load()
}

// TxFailureRate returns the number of expired transactions divided by total number of transactions
func (st *TxStatsTracker) TxFailureRate() float64 {
	count := st.count.Load()
	if count == 0 {
		return 0
	}
	return float64(st.failures.Load()) / float64(count)
}

func (st *TxStatsTracker) String() string {
	b := strings.Builder{}
	for _, s := range []*circonusllhist.Histogram{st.TTF, st.TTE, st.TTS} {
		b.WriteString(fmt.Sprintf("%+v\n", s.DecStrings()))
	}
	return b.String()
}

func (st *TxStatsTracker) Digest() string {
	t := table.NewWriter()
	t.AppendHeader(table.Row{
		"total TX",
		"failure rate",
		"avg TTF",
		"median TTF",
		"max TTF",
		"avg TTE",
		"median TTE",
		"max TTE",
		"avg TTS",
		"median TTS",
		"max TTS"})
	t.AppendRow(table.Row{
		st.TotalTxSubmited(),
		st.TxFailureRate(),
		st.TTF.Mean(),
		st.TTF.ValueAtQuantile(0.5),
		st.TTF.Max(),
		st.TTE.Mean(),
		st.TTE.ValueAtQuantile(0.5),
		st.TTE.Max(),
		st.TTS.Mean(),
		st.TTS.ValueAtQuantile(0.5),
		st.TTS.Max(),
	})
	return t.Render()
}
