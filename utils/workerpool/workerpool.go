package workerpool

import (
	"github.com/gammazero/workerpool"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// WorkerPool is a utility structure which allows wrapping WorkerPool into module.Startable and module.ReadyDoneAware
// interfaces and allows to use this structure as component.
type WorkerPool struct {
	*component.ComponentManager
	*workerpool.WorkerPool
}

// New performs initialization of worker pool and setups component manager
// which will shutdown workers when requested to quit.
func New(maxWorkers int) *WorkerPool {
	pool := workerpool.New(maxWorkers)
	builder := component.NewComponentManagerBuilder()
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()
		<-ctx.Done()
		pool.StopWait() // wait till all workers exit
	})

	return &WorkerPool{
		ComponentManager: builder.Build(),
		WorkerPool:       pool,
	}
}
