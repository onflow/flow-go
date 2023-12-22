package checker

import (
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type Engine struct {
	*component.ComponentManager
}

func NewEngine(core *Core) *Engine {
	e := &Engine{}

	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			err := core.runLoop(ctx, DefaultTimeInterval)
			if err != nil {
				ctx.Throw(err)
			}
		}).
		Build()

	return e
}
