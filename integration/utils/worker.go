package utils

import (
	"context"
	"sync"
	"time"
)

type workFunc func(workerID int)

type Worker struct {
	workerID int
	interval time.Duration
	work     workFunc

	ctx    context.Context
	cancel context.CancelFunc

	wg *sync.WaitGroup
}

func NewWorker(workerID int, interval time.Duration, work workFunc) Worker {
	ctx, cancel := context.WithCancel(context.Background())

	return Worker{
		workerID: workerID,
		interval: interval,
		work:     work,

		ctx:    ctx,
		cancel: cancel,

		wg: &sync.WaitGroup{},
	}
}

func (w *Worker) Start() {
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()

		t := time.NewTicker(w.interval)
		defer t.Stop()
		for ; ; <-t.C {
			select {
			case <-w.ctx.Done():
				return
			default:
			}

			w.wg.Add(1)
			go func() {
				defer w.wg.Done()
				w.work(w.workerID)
			}()
		}
	}()
}

func (w *Worker) Stop() {
	w.cancel()
	// After this no new workers will be spawn and last worker have finished
	w.wg.Wait()
}
