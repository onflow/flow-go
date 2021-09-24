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
	// Irrecoverable error handling is built using a heirachy of contexts. This
	// simplifies the signalling by separating the shutdown logic from the
	// ReadyDoneAware interface. This context is mandatory and is used to both
	// pass the irrecoverable error channel and is the only way to shutdown child
	// components.
	ctx, cancel := context.WithCancel(context.Background())

	// module.ComponentFactory encapsulates all of the component building logic
	// required before running Start()
	starts := 0
	componentFactory := func() (module.Component, error) {
		starts++
		return NewExampleComponent(starts), nil
	}

	// irrecoverable.OnError is run when the component throws an irrecoverable
	// error, or after cancelling its context and waiting for its Done channel
	// to close.
	// This is the place to inspect the specific error returned to determine if
	// it's appropriate to restart the component, or shutdown/panic. It's also
	// where logic to perform additional cleanup or any alerting/telemetry goes.
	onError := func(err error, triggerRestart func()) {
		// check to make sure it's safe to restart the component
		var tmp TemporaryError
		if errors.As(err, &tmp) {
			fmt.Printf("Restarting component after fatal error: %v\n", err)
			triggerRestart()
			return
		}

		// it's not safe to restart, shutdown instead
		fmt.Printf("An unrecoverable error occurred: %v\n", err)

		// cleanly shutdown other components. the component itself is cancelled
		// within RunComponent.
		cancel()
	}

	// run the component. this will return an error immediately if it fails to
	// start the component, otherwise it will block (and restart) until:
	// * onError is called and does not call the triggerRestart() callback
	// * ctx is cancelled
	// If the component needs to start additional subcomponents, the same
	// approach can be applied by using the same context and creating new
	// ComponentFactory and OnError handlers. This creates a hierarchical
	// model where the parent components supervise and restart their children
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

///////////////////////////////////////////////////////////////////////////////
//                                                                           //
// The following is an example of a standard Component which implements      //
// ReadyDoneAware and Startable. Nothing here is required except for some    //
// implementation of the interface methods. Ready and Done should return     //
// their respective channels, and perform no startup/shutdown logic.         //
//                                                                           //
///////////////////////////////////////////////////////////////////////////////

type ExampleComponent struct {
	id int
	lm *lifecycle.LifecycleManager
}

func NewExampleComponent(id int) *ExampleComponent {
	return &ExampleComponent{
		id: id,
		lm: lifecycle.NewLifecycleManager(),
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
				if c.id%2 == 0 {
					ctx.Throw(errors.New("Fatal! No restarts"))
				}
				ctx.Throw(TemporaryError{"Restart me!"})
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
