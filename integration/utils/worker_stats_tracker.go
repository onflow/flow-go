package utils

import (
	"fmt"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/table"
)

// WorkerStatsTracker keeps track of worker stats
type WorkerStatsTracker struct {
	mux              sync.Mutex
	workers          int
	txsSent          int
	txsSentPerSecond map[int64]int // tracks txs sent at the timestamp in seconds

	printer *Worker
}

// NewWorkerStatsTracker returns a new instance of WorkerStatsTracker
func NewWorkerStatsTracker() *WorkerStatsTracker {
	return &WorkerStatsTracker{
		txsSentPerSecond: make(map[int64]int),
	}
}

// StartPrinting starts reporting of worker stats
func (st *WorkerStatsTracker) StartPrinting(interval time.Duration) {
	printer := NewWorker(0, interval, func(_ int) { fmt.Println(st.Digest()) })
	st.printer = &printer
	st.printer.Start()
}

// StopPrinting stops reporting of worker stats
func (st *WorkerStatsTracker) StopPrinting() {
	st.printer.Stop()
}

func (st *WorkerStatsTracker) AddWorker() {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.workers++
}

func (st *WorkerStatsTracker) AddTxSent() {
	now := time.Now().Unix()

	st.mux.Lock()
	defer st.mux.Unlock()

	st.txsSent++
	st.txsSentPerSecond[now]++
}

// AvgTPSBetween returns the average transactions per second TPS between the two time points
func (st *WorkerStatsTracker) AvgTPSBetween(start, stop time.Time) float64 {
	sum := 0
	toDelete := make([]int64, 0)

	st.mux.Lock()
	defer st.mux.Unlock()
	for timestamp, count := range st.txsSentPerSecond {
		if timestamp < start.Unix() {
			toDelete = append(toDelete, timestamp)
			continue
		}
		sum += count
	}

	for _, timestamp := range toDelete {
		delete(st.txsSentPerSecond, timestamp)
	}

	diff := stop.Sub(start)

	return float64(sum) / diff.Seconds()
}

func (st *WorkerStatsTracker) Digest() string {
	t := table.NewWriter()
	t.AppendHeader(table.Row{
		"workers",
		"total TXs sent",
		"Avg TPS (last 10s)",
	})
	t.AppendRow(table.Row{
		st.workers,
		st.txsSent,
		// use 11 seconds to correct for rounding in buckets
		st.AvgTPSBetween(time.Now().Add(-11*time.Second), time.Now()),
	})
	return t.Render()
}
