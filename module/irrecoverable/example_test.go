package irrecoverable_test

import (
	"context"
	"errors"
	"fmt"
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

		return NewExampleComponent(starts, err), nil
	}

	var onError module.OnError
	onError = func(err error, triggerRestart func()) {
		// check to make sure it's safe to restart the component
		var tmp TemporaryError
		if errors.As(err, &tmp) {
			fmt.Printf("Restarting component after fatal error: %v\n", err)
			triggerRestart()
			return
		}

		// it's not safe to restart, shutdown instead
		fmt.Printf("An unrecoverable error occurred: %v\n", err)
		// shutdown other components. it might make sense to just panic here
		// depending on the circumstances
		cancel()
	}

	// run the component. this will block until onError fails to restart the component
	// or its context is cancelled
	err := module.RunComponent(ctx, componentFactory, onError)
	if err != nil {
		fmt.Printf("Error returned from RunComponent: %v\n", err)
	}

	// Output:
	// [Component 1] Starting up
	// [Component 1] Running cleanup
	// Restarting component after fatal error: Restart me!
	// [Component 2] Starting up
	// [Component 2] Running cleanup
	// An unrecoverable error occurred: Fatal! No restarts
	// Error returned from RunComponent: context canceled
}

type ExampleComponent struct {
	id  int
	err error
	lm  *lifecycle.LifecycleManager
}

func NewExampleComponent(id int, err error) *ExampleComponent {
	return &ExampleComponent{
		id:  id,
		err: err,
		lm:  lifecycle.NewLifecycleManager(),
	}
}

// start the component and register its shutdown handler
// this component will throw an error after 20ms to demonstrate the error handling
func (c *ExampleComponent) Start(ctx irrecoverable.SignalerContext) error {
	c.lm.OnStart(func() {
		go c.shutdownOnCancel(ctx)

		c.printMsg("Starting up")

		// do some setup...

		go func() {
			select {
			case <-time.After(20 * time.Millisecond):
				// encounter irrecoverable error
				ctx.ThrowError(c.err)
			case <-ctx.Done():
				c.printMsg("Cancelled by parent")
			}
		}()
	})

	return nil
}

// run the lifecycle OnStop when the component's context is cancelled
func (c *ExampleComponent) shutdownOnCancel(ctx context.Context) {
	<-ctx.Done()

	c.lm.OnStop(func() {
		c.printMsg("Running cleanup")
		// do some cleanup...
	})
}

// simply return the Started channel
// all startup processing is done in Start()
func (c *ExampleComponent) Ready() <-chan struct{} {
	return c.lm.Started()
}

// simply return the Stopped channel
// all shutdown processing is done in shutdownOnCancel()
func (c *ExampleComponent) Done() <-chan struct{} {
	return c.lm.Stopped()
}

func (c *ExampleComponent) printMsg(msg string) {
	fmt.Printf("[Component %d] %s\n", c.id, msg)
}
