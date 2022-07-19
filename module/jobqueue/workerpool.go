package jobqueue

import (
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
)

// WorkerPool implements the jobqueue.Worker interface, and wraps the processing to make it
// compatible with the Component interface.
type WorkerPool struct {
	component.Component

	cm        *component.ComponentManager
	processor JobProcessor
	notify    NotifyDone
	ch        chan module.Job
}

// JobProcessor is called by the worker to execute each job. It should only return when the job has
// completed, either successfully or after performing any failure handling.
// It takes 3 arguments:
// - irrecoverable.SignalerContext: this is used to signal shutdown to the worker and throw any
//   irrecoverable errors back to the parent. The signaller context is passed in from consumer's
//   Start method
// - module.Job: the job to be processed. The processor is responsible for decoding into the
//   expected format.
// - func(): Call this closure after the job is considered complete. This is a convenience method
//   that avoid needing to a separate ProcessingNotifier for simple usecases. If a different method
//   is used to signal jobs are done to the consumer, this function can be ignored.
type JobProcessor func(irrecoverable.SignalerContext, module.Job, func())

// NotifyDone should be the consumer's NotifyJobIsDone method, or a wrapper for that method. It is
// wrapped in a closure and added as an argument to the JobProcessor to notify the consumer that
// the job is done.
type NotifyDone func(module.JobID)

// NewWorkerPool returns a new WorkerPool
func NewWorkerPool(processor JobProcessor, notify NotifyDone, workers uint64) *WorkerPool {
	w := &WorkerPool{
		processor: processor,
		notify:    notify,
		ch:        make(chan module.Job),
	}

	builder := component.NewComponentManagerBuilder()

	for i := uint64(0); i < workers; i++ {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			w.workerLoop(ctx)
		})
	}

	w.cm = builder.Build()
	w.Component = w.cm

	return w
}

// Run executes the worker's JobProcessor for the provided job.
// Run is non-blocking.
func (w *WorkerPool) Run(job module.Job) error {
	// don't accept new jobs after shutdown is signalled
	if util.CheckClosed(w.cm.ShutdownSignal()) {
		return nil
	}

	select {
	case <-w.cm.ShutdownSignal():
		return nil
	case w.ch <- job:
	}

	return nil
}

// workerLoop processes incoming jobs passed via the Run method. The job execution is wrapped in a
// goroutine to support passing the worker's irrecoverable.SignalerContext into the processor.
func (w *WorkerPool) workerLoop(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-w.ch:
			w.processor(ctx, job, func() {
				w.notify(job.ID())
			})
		}
	}
}
