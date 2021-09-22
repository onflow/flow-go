package irrecoverable_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/lifecycle"
)

type TemporaryError struct {
	message string
}

func (s TemporaryError) Error() string {
	return s.message
}

func Example() {

	ctx, cancel := context.WithCancel(context.Background())

	starts := 0
	var componentFactory module.ComponentFactory
	componentFactory = func() (module.Component, error) {
		var err error
		starts++
		if starts > 1 {
			err = errors.New("Fatal! No restarts")
		} else {
			err = TemporaryError{"Restart me!"}
		}
		component := &ExampleComponent{
			id:  starts,
			err: err,
			lm:  lifecycle.NewLifecycleManager(),
		}

		return component, nil
	}

	var onError module.OnError
	onError = func(err error, triggerRestart func()) {
		var tmp TemporaryError
		if errors.As(err, &tmp) {
			fmt.Printf("Restarting component after fatal error: %v\n", err)
			triggerRestart()
			return
		}
		fmt.Printf("An unrecoverable error occurred: %v\n", err)
		// shutdown other components
		cancel()
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := module.RunComponent(ctx, componentFactory, onError)
		if err != nil {
			fmt.Printf("Error returned from RunComponent 1: %v\n", err)
		}
	}()

	wg.Wait()

	// Output:
	// Starting up 1
	// Running cleanup 1
	// Restarting component after fatal error: Restart me!
	// Starting up 2
	// Running cleanup 2
	// An unrecoverable error occurred: Fatal! No restarts
	// Error returned from RunComponent 1: context canceled
}

type ExampleComponent struct {
	id  int
	err error
	lm  *lifecycle.LifecycleManager
}

func (c *ExampleComponent) Start(ctx irrecoverable.SignalerContext) error {
	c.lm.OnStart(func() {
		fmt.Printf("Starting up %d\n", c.id)

		go c.shutdownOnCancel(ctx)
		go func() {
			<-time.After(20 * time.Millisecond)
			ctx.ThrowError(c.err)
		}()
	})
	return nil
}

func (c *ExampleComponent) shutdownOnCancel(ctx context.Context) {
	<-ctx.Done()

	c.lm.OnStop(func() {
		fmt.Printf("Running cleanup %d\n", c.id)
	})
}

func (c *ExampleComponent) Ready() <-chan struct{} {
	return c.lm.Started()
}

func (c *ExampleComponent) Done() <-chan struct{} {
	return c.lm.Stopped()
}
