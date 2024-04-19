package benchmark

import (
	"context"
	"sync"
	"time"

	"github.com/VividCortex/ewma"
	"github.com/rs/zerolog"
)

type WorkerStats struct {
	Workers                  int
	TxsSent                  int
	TxsTimedout              int
	TxsExecuted              int
	TxsFailed                int
	TxsSentMovingAverage     float64
	TxsExecutedMovingAverage float64
}

// WorkerStatsTracker keeps track of worker stats
type WorkerStatsTracker struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mux             sync.Mutex
	stats           WorkerStats
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

	lastStats := st.GetStats()
	for {
		select {
		case <-t.C:
			stats := st.GetStats()
			st.updateEWMAonce(lastStats, stats)
			lastStats = stats
		case <-st.ctx.Done():
			return
		}
	}
}

// updateEWMAonce updates all Exponentially Weighted Moving Averages with the given stats.
func (st *WorkerStatsTracker) updateEWMAonce(lastStats, stats WorkerStats) {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.txsSentEWMA.Add(float64(stats.TxsSent - lastStats.TxsSent))
	st.stats.TxsSentMovingAverage = st.txsSentEWMA.Value()
	st.txsExecutedEWMA.Add(float64(stats.TxsExecuted - lastStats.TxsExecuted))
	st.stats.TxsExecutedMovingAverage = st.txsExecutedEWMA.Value()
}

func (st *WorkerStatsTracker) Stop() {
	st.cancel()
	st.wg.Wait()
}

func (st *WorkerStatsTracker) IncTxTimedOut() {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.stats.TxsTimedout++
}

func (st *WorkerStatsTracker) IncTxExecuted() {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.stats.TxsExecuted++
}

func (st *WorkerStatsTracker) IncTxFailed() {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.stats.TxsFailed++
}

func (st *WorkerStatsTracker) AddWorkers(i int) {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.stats.Workers += i
}

func (st *WorkerStatsTracker) IncTxSent() {
	st.mux.Lock()
	defer st.mux.Unlock()

	st.stats.TxsSent++
}

func (st *WorkerStatsTracker) GetStats() WorkerStats {
	st.mux.Lock()
	defer st.mux.Unlock()

	return st.stats
}

func NewPeriodicStatsLogger(
	ctx context.Context,
	st *WorkerStatsTracker,
	log zerolog.Logger,
) *Worker {
	w := NewWorker(
		ctx,
		0,
		3*time.Second,
		func(workerID int) {
			stats := st.GetStats()
			log.Info().
				Int("Workers", stats.Workers).
				Int("TxsSent", stats.TxsSent).
				Int("TxsTimedout", stats.TxsTimedout).
				Int("TxsExecuted", stats.TxsExecuted).
				Float64("TxsSentMovingAverage", stats.TxsSentMovingAverage).
				Float64("TxsExecutedMovingAverage", stats.TxsExecutedMovingAverage).
				Msg("worker stats")
		},
	)

	return w
}
