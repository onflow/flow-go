package jobqueue

import (
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
)

// WrappedWorker implements the jobqueue.Worker interface, and wraps the processing to make it
// compatible with the Component interface.
type WrappedWorker struct {
	component.Component

	cm        *component.ComponentManager
	processor JobProcessor
	notify    NotifyDone
	ch        chan *work
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

type work struct {
	job  module.Job
	done chan struct{}
}

// NewWrappedWorker returns a new WrappedWorker
func NewWrappedWorker(processor JobProcessor, notify NotifyDone, workers uint64) *WrappedWorker {
	w := &WrappedWorker{
		processor: processor,
		notify:    notify,
		ch:        make(chan *work),
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
func (w *WrappedWorker) Run(job module.Job) error {
	// don't accept new jobs after shutdown is signalled
	if util.CheckClosed(w.cm.ShutdownSignal()) {
		return nil
	}

	work := &work{
		job:  job,
		done: make(chan struct{}),
	}

	// the consumer should only call Run if there are available workers, but just in case block
	select {
	case <-w.cm.ShutdownSignal():
		return nil
	case w.ch <- work:
	}

	// Run is expected to be blocking, so wait until the work is done.
	select {
	case <-w.cm.ShutdownSignal():
		return nil
	case <-work.done:
	}

	return nil
}

// workerLoop processes incoming jobs passed via the Run method. The job execution is wrapped in a
// goroutine to support passing the worker's irrecoverable.SignalerContext into the processor.
func (w *WrappedWorker) workerLoop(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		case work := <-w.ch:
			w.processor(ctx, work.job, func() {
				w.notify(work.job.ID())
			})
			close(work.done)
		}
	}
}
