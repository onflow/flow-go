package irrecoverable_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

var ErrTriggerRestart = errors.New("restart me")
var ErrNoRestart = errors.New("fatal, no restarts")

func Example() {
	// a context is mandatory in order to call RunComponent
	ctx, cancel := context.WithCancel(context.Background())

	// component.ComponentFactory encapsulates all of the component building logic
	// required before running Start()
	starts := 0
	componentFactory := func() (component.Component, error) {
		starts++
		return NewExampleComponent(starts), nil
	}

	// this is the place to inspect the encountered error and implement the appropriate error
	// handling behaviors, e.g. restarting the component, firing an alert to pagerduty, etc ...
	// the shutdown of the component is handled for you by RunComponent, but you may consider
	// performing additional cleanup here
	onError := func(err error, triggerRestart func()) {
		// check the error type to decide whether to restart or shutdown
		if errors.Is(err, ErrTriggerRestart) {
			fmt.Printf("Restarting component after fatal error: %v\n", err)
			triggerRestart()
			return
		} else {
			fmt.Printf("An unrecoverable error occurred: %v\n", err)
			// shutdown other components. it might also make sense to just panic here
			// depending on the circumstances
			cancel()
		}

	}

	// run the component. this is a blocking call, and will return with an error if the
	// first startup or any subsequent restart attempts fails or the context is canceled
	err := component.RunComponent(ctx, componentFactory, onError)
	if err != nil {
		fmt.Printf("Error returned from RunComponent: %v\n", err)
	}

	// Output:
	// [Component 1] Starting up
	// [Component 1] Shutting down
	// Restarting component after fatal error: restart me
	// [Component 2] Starting up
	// [Component 2] Shutting down
	// An unrecoverable error occurred: fatal, no restarts
	// Error returned from RunComponent: context canceled
}

// ExampleComponent is an example of a typical component
type ExampleComponent struct {
	id      int
	started chan struct{}
	ready   sync.WaitGroup
	done    sync.WaitGroup
}

func NewExampleComponent(id int) *ExampleComponent {
	return &ExampleComponent{
		id:      id,
		started: make(chan struct{}),
	}
}

// start the component and register its shutdown handler
// this component will throw an error after 20ms to demonstrate the error handling
func (c *ExampleComponent) Start(ctx irrecoverable.SignalerContext) {
	c.printMsg("Starting up")

	// do some setup...

	c.ready.Add(2)
	c.done.Add(2)

	go func() {
		c.ready.Done()
		defer c.done.Done()

		<-ctx.Done()

		c.printMsg("Shutting down")
		// do some cleanup...
	}()

	go func() {
		c.ready.Done()
		defer c.done.Done()

		select {
		case <-time.After(20 * time.Millisecond):
			// encounter irrecoverable error
			if c.id > 1 {
				ctx.Throw(ErrNoRestart)
			} else {
				ctx.Throw(ErrTriggerRestart)
			}
		case <-ctx.Done():
			c.printMsg("Cancelled by parent")
		}
	}()

	close(c.started)
}

// simply return the Started channel
// all startup processing is done in Start()
func (c *ExampleComponent) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		<-c.started
		c.ready.Wait()
		close(ready)
	}()
	return ready
}

// simply return the Stopped channel
// all shutdown processing is done in shutdownOnCancel()
func (c *ExampleComponent) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-c.started
		c.done.Wait()
		close(done)
	}()
	return done
}

func (c *ExampleComponent) printMsg(msg string) {
	fmt.Printf("[Component %d] %s\n", c.id, msg)
}
