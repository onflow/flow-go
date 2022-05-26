package utils

import (
	"context"
	"time"
)

type workFunc func(workerID int)

type Worker struct {
	workerID int
	interval time.Duration
	work     workFunc
	ticker   *time.Ticker

	ctx    context.Context
	cancel context.CancelFunc
}

func NewWorker(workerID int, interval time.Duration, work workFunc) Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return Worker{
		workerID: workerID,
		interval: interval,
		work:     work,

		ctx:    ctx,
		cancel: cancel,
	}
}

func (w *Worker) Start() {
	w.ticker = time.NewTicker(w.interval)
	defer w.ticker.Stop()

	go func() {
		for ; ; <-w.ticker.C {
			select {
			case <-w.ctx.Done():
				return
			default:
			}
			go w.work(w.workerID)
		}
	}()
}

func (w *Worker) Stop() {
	w.cancel()
}
