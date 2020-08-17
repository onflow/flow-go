package queue

type Work struct {
	f func() // The function that we will call.
}

type Worker struct {
	ID            int            // The ID of the worker.
	WorkerChannel chan chan Work // The channel for the workers.
	Channel       chan Work      // The channel to receive work.
	End           chan bool      // Has the worker stopped?
}

type Collector struct {
	Work chan Work // The channel to receive work.
	End  chan bool // Has the collector stopped?
}

var WorkerChannel = make(chan chan Work)

func StartDispatcher(workerCount int) Collector {
	var i int
	var workers []Worker
	input := make(chan Work) // channel to receive work
	end := make(chan bool)   // channel to spin down workers
	collector := Collector{Work: input, End: end}

	for i < workerCount {
		i++
		worker := Worker{
			ID:            i,
			Channel:       make(chan Work),
			WorkerChannel: WorkerChannel,
			End:           make(chan bool)}
		worker.Start()
		workers = append(workers, worker) // store worker
	}

	// start collector
	go func() {
		for {
			select {
			case <-end:
				for _, w := range workers {
					w.Stop() // stop worker
				}
				return
			case work := <-input:
				worker := <-WorkerChannel // wait for available channel
				worker <- work            // dispatch work to worker
			}
		}
	}()

	return collector
}

// start worker
func (w *Worker) Start() {
	go func() {
		for {
			w.WorkerChannel <- w.Channel
			select {
			case job := <-w.Channel:
				job.f()
			case <-w.End:
				return
			}
		}
	}()
}

// end worker
func (w *Worker) Stop() {
	w.End <- true
}
