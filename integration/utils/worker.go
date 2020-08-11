package utils

import (
	"time"
)

type Worker struct {
	workerID int
	interval time.Duration
	work     func(workerID int)
	ticker   *time.Ticker
	quit     chan struct{}
}

func NewWorker(workerID int, interval time.Duration, work func(workerID int)) Worker {
	return Worker{
		workerID: workerID,
		interval: interval,
		work:     work,
	}
}

func (w *Worker) Start() {
	w.ticker = time.NewTicker(w.interval)
	w.quit = make(chan struct{})

	go func() {
		for {
			select {
			case <-w.ticker.C:
				go w.work(w.workerID)
			case <-w.quit:
				w.ticker.Stop()
				return
			}
		}
	}()
}

func (w *Worker) Stop() {
	close(w.quit)
}
