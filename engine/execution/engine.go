package execution

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
)

// Engine manages execution of transactions
type Engine struct {
	log     zerolog.Logger
	conduit network.Conduit
	me      module.Local
	wg      *sync.WaitGroup
}

func New(logger zerolog.Logger, net module.Network, me module.Local) (*Engine, error) {

	eng := Engine{
		log: logger,
		me:  me,
		wg:  &sync.WaitGroup{},
	}

	con, err := net.Register(engine.Execution, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	eng.conduit = con

	return &eng, nil
}

// Ready returns a channel that will close when the engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when the engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	return done
}

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	var err error
	switch event.(type) {
	default:
		err = errors.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}
	return nil
}
