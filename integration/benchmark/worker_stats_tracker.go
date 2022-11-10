package benchmark

import (
	"context"
	"sync"
	"time"

	"github.com/VividCortex/ewma"
	"github.com/rs/zerolog"
)

type workerStats struct {
	workers                  int
	txsSent                  int
	txsExecuted              int
	txsSentMovingAverage     float64
	txsExecutedMovingAverage float64
}

// WorkerStatsTracker keeps track of worker stats
type WorkerStatsTracker struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mux             sync.Mutex
	stats           workerStats
	txsSentEWMA     ewma.MovingAverage
	txsExecutedEWMA ewma.MovingAverage
}

const defaultMovingAverageAge = 10

// NewWorkerStatsTracker returns a new instance of WorkerStatsTracker
func NewWorkerStatsTracker(ctx context.Context) *WorkerStatsTracker {
	ctx, cancel := context.WithCancel(ctx)
	st := &WorkerStatsTracker{
		ctx:             ctx,
		cancel:          cancel,
		txsSentEWMA:     ewma.NewMovingAverage(defaultMovingAverageAge),
		txsExecutedEWMA: ewma.NewMovingAverage(defaultMovingAverageAge),
	}

	st.wg.Add(1)
	go st.updateEWMAforever()
	return st
}

func (st *WorkerStatsTracker) updateEWMAforever() {
	defer st.wg.Done()

	t := time.NewTicker(time.Second)
	defer t.Stop()

	lastStats := st.getStats()
	for {
		select {
		case <-t.C:
			stats := st.getStats()
			st.updateEWMAonce(lastStats, stats)
			lastStats = stats
		case <-st.ctx.Done():
			return
		}
	}
}

// updateEWMAonce updates all Exponentially Weighted Moving Averages with the given stats.
func (st *WorkerStatsTracker) updateEWMAonce(lastStats, stats workerStats) {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.txsSentEWMA.Add(float64(stats.txsSent - lastStats.txsSent))
	st.txsExecutedEWMA.Add(float64(stats.txsExecuted - lastStats.txsExecuted))
}

func (st *WorkerStatsTracker) Stop() {
	st.cancel()
	st.wg.Wait()
}

func (st *WorkerStatsTracker) IncTxExecuted() {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.stats.txsExecuted++
}

func (st *WorkerStatsTracker) GetTxExecuted() int {
	st.mux.Lock()
	defer st.mux.Unlock()

	return st.stats.txsExecuted
}

func (st *WorkerStatsTracker) AddWorkers(i int) {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.stats.workers += i
}

func (st *WorkerStatsTracker) IncTxSent() {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.stats.txsSent++
}

func (st *WorkerStatsTracker) GetTxSent() int {
	st.mux.Lock()
	defer st.mux.Unlock()

	return st.stats.txsSent
}

func (st *WorkerStatsTracker) getStats() workerStats {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.stats.txsSentMovingAverage = st.txsSentEWMA.Value()
	st.stats.txsExecutedMovingAverage = st.txsExecutedEWMA.Value()
	return st.stats
}

func NewPeriodicStatsLogger(st *WorkerStatsTracker, log zerolog.Logger) *Worker {
	w := NewWorker(0, 1*time.Second, func(workerID int) {
		stats := st.getStats()
		log.Info().
			Int("workers", stats.workers).
			Int("txsSent", stats.txsSent).
			Int("txsExecuted", stats.txsExecuted).
			Float64("txsSentEWMA", stats.txsSentMovingAverage).
			Float64("txsExecutedEWMA", stats.txsExecutedMovingAverage).
			Msg("worker stats")
	})

	return w
}
