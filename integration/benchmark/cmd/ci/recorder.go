package main

import (
	"context"
	"sync"
	"time"

	"github.com/onflow/flow-go/integration/benchmark"
)

type RawTPSRecord struct {
	Timestamp     time.Time
	OffsetSeconds float64

	InputTPS    float64
	OutputTPS   float64
	TimedoutTPS float64
	ErrorTPS    float64

	InflightTxs int
}

type Status string

const (
	StatusUnknown Status = "UNKNOWN"
	StatusSuccess Status = "SUCCESS"
	StatusFailure Status = "FAILURE"
)

// BenchmarkResults is used for uploading data to BigQuery.
type BenchmarkResults struct {
	StartTime       time.Time
	StopTime        time.Time
	DurationSeconds float64

	Status Status

	RawTPS []RawTPSRecord
}

type TPSRecorder struct {
	BenchmarkResults

	lastStats benchmark.WorkerStats
	lastTs    time.Time

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	stopOnce *sync.Once
}

func NewTPSRecorder(
	ctx context.Context,
	workerStatsTracker *benchmark.WorkerStatsTracker,
	statInterval time.Duration,
) *TPSRecorder {
	ctx, cancel := context.WithCancel(ctx)

	r := &TPSRecorder{
		BenchmarkResults: BenchmarkResults{
			Status:    StatusUnknown,
			StartTime: time.Now(),
		},
		done:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,

		stopOnce: &sync.Once{},
	}

	go func() {
		t := time.NewTicker(statInterval)
		defer t.Stop()

		defer close(r.done)

		for {
			r.record(time.Now(), workerStatsTracker.GetStats())
			select {
			case <-t.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	return r
}

func (r *TPSRecorder) Stop() {
	r.stopOnce.Do(func() {
		r.cancel()
		<-r.done

		r.StopTime = time.Now()
		r.DurationSeconds = r.StopTime.Sub(r.StartTime).Seconds()
		if r.Status == StatusUnknown {
			r.Status = StatusSuccess
		}
	})
}

func (r *TPSRecorder) SetStatus(status Status) {
	r.Status = status
}

func (r *TPSRecorder) record(nowTs time.Time, stats benchmark.WorkerStats) {
	if !r.lastTs.IsZero() {
		r.RawTPS = append(r.RawTPS, r.statsToRawTPS(nowTs, stats))
	}

	r.lastStats = stats
	r.lastTs = nowTs
}

func (r *TPSRecorder) statsToRawTPS(nowTs time.Time, stats benchmark.WorkerStats) RawTPSRecord {
	timeDiff := nowTs.Sub(r.lastTs).Seconds()

	return RawTPSRecord{
		Timestamp:     nowTs,
		OffsetSeconds: nowTs.Sub(r.StartTime).Seconds(),

		InputTPS:    float64(stats.TxsSent-r.lastStats.TxsSent) / timeDiff,
		OutputTPS:   float64(stats.TxsExecuted-r.lastStats.TxsExecuted) / timeDiff,
		TimedoutTPS: float64(stats.TxsTimedout-r.lastStats.TxsTimedout) / timeDiff,
		ErrorTPS:    float64(stats.TxsFailed-r.lastStats.TxsFailed) / timeDiff,

		InflightTxs: stats.TxsSent - stats.TxsExecuted,
	}
}
