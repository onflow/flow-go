package component_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

var ErrFatal = errors.New("fatal")
var ErrCouldNotCreateComponent = errors.New("failed to create component")

var _ component.Component = (*StartupErroringComponent)(nil)
var _ component.Component = (*RunningErroringComponent)(nil)
var _ component.Component = (*ShutdownErroringComponent)(nil)
var _ component.Component = (*ConcurrentErroringComponent)(nil)

type StartupErroringComponent struct {
	ready chan struct{}
	done  chan struct{}
}

func NewStartupErroringComponent() *StartupErroringComponent {
	return &StartupErroringComponent{
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
}

func (c *StartupErroringComponent) Start(ctx irrecoverable.SignalerContext) {
	go func() {
		defer close(c.done)

		// throw fatal error during startup
		ctx.Throw(ErrFatal)

		// close(c.ready)
	}()
}

func (c *StartupErroringComponent) Ready() <-chan struct{} {
	return c.ready
}

func (c *StartupErroringComponent) Done() <-chan struct{} {
	return c.done
}

type RunningErroringComponent struct {
	ready    chan struct{}
	done     chan struct{}
	duration time.Duration
}

func NewRunningErroringComponent(duration time.Duration) *RunningErroringComponent {
	return &RunningErroringComponent{
		ready:    make(chan struct{}),
		done:     make(chan struct{}),
		duration: duration,
	}
}

func (c *RunningErroringComponent) Start(ctx irrecoverable.SignalerContext) {
	go func() {
		defer close(c.done)
		close(c.ready)

		// do some work...
		select {
		case <-time.After(c.duration):
		case <-ctx.Done():
			return
		}

		// throw fatal error
		ctx.Throw(ErrFatal)
	}()
}

func (c *RunningErroringComponent) Ready() <-chan struct{} {
	return c.ready
}

func (c *RunningErroringComponent) Done() <-chan struct{} {
	return c.done
}

type ShutdownErroringComponent struct {
	ready    sync.WaitGroup
	done     sync.WaitGroup
	started  chan struct{}
	duration time.Duration
}

func NewShutdownErroringComponent(duration time.Duration) *ShutdownErroringComponent {
	return &ShutdownErroringComponent{
		started:  make(chan struct{}),
		duration: duration,
	}
}

func (c *ShutdownErroringComponent) Start(ctx irrecoverable.SignalerContext) {
	c.ready.Add(2)
	c.done.Add(2)

	go func() {
		c.ready.Done()
		defer c.done.Done()

		// do some work...
		select {
		case <-time.After(c.duration):
		case <-ctx.Done():
			return
		}

		// encounter fatal error
		ctx.Throw(ErrFatal)
	}()

	go func() {
		c.ready.Done()
		defer c.done.Done()

		// wait for shutdown signal triggered by fatal error
		<-ctx.Done()

		// encounter error during shutdown
		ctx.Throw(ErrFatal)
	}()

	close(c.started)
}

func (c *ShutdownErroringComponent) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		<-c.started
		c.ready.Wait()
		close(ready)
	}()
	return ready
}

func (c *ShutdownErroringComponent) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-c.started
		c.done.Wait()
		close(done)
	}()
	return done
}

type ConcurrentErroringComponent struct {
	ready    sync.WaitGroup
	done     sync.WaitGroup
	started  chan struct{}
	duration time.Duration
}

func NewConcurrentErroringComponent(duration time.Duration) *ConcurrentErroringComponent {
	return &ConcurrentErroringComponent{
		started:  make(chan struct{}),
		duration: duration,
	}
}

func (c *ConcurrentErroringComponent) Start(ctx irrecoverable.SignalerContext) {
	c.ready.Add(2)
	c.done.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			c.ready.Done()
			defer c.done.Done()

			// do some work...
			select {
			case <-time.After(c.duration):
			case <-ctx.Done():
				return
			}

			// encounter fatal error
			ctx.Throw(ErrFatal)
		}()
	}

	close(c.started)
}

func (c *ConcurrentErroringComponent) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		<-c.started
		c.ready.Wait()
		close(ready)
	}()
	return ready
}

func (c *ConcurrentErroringComponent) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-c.started
		c.done.Wait()
		close(done)
	}()
	return done
}

type NonErroringComponent struct {
	ready    chan struct{}
	done     chan struct{}
	duration time.Duration
}

func NewNonErroringComponent(duration time.Duration) *NonErroringComponent {
	return &NonErroringComponent{
		ready:    make(chan struct{}),
		done:     make(chan struct{}),
		duration: duration,
	}
}

func (c *NonErroringComponent) Start(ctx irrecoverable.SignalerContext) {
	go func() {
		defer close(c.done)

		// do some work...
		select {
		case <-time.After(c.duration):
		case <-ctx.Done():
			return
		}

		// exit gracefully
	}()
}

func (c *NonErroringComponent) Ready() <-chan struct{} {
	return c.ready
}

func (c *NonErroringComponent) Done() <-chan struct{} {
	return c.done
}

//tests that after starting a component that generates, an expected error is received
func TestRunComponentStartupError(t *testing.T) {
	componentFactory := func() (component.Component, error) {
		return NewStartupErroringComponent(), nil
	}

	called := false
	onError := func(err error) component.ErrorHandlingResult {
		called = true
		require.ErrorIs(t, err, ErrFatal)  //check that really got the fatal error we were expecting
		return component.ErrorHandlingStop //stop component after receiving the error (don't restart it)
	}

	//irrelevant what context we use - test won't be using it
	err := component.RunComponent(context.Background(), componentFactory, onError)
	require.ErrorIs(t, err, ErrFatal)
	require.True(t, called)
}

//tests repeatedly restarting a component during an error that occurs while shutting down
func TestRunComponentShutdownError(t *testing.T) {
	componentFactory := func() (component.Component, error) {
		//shutdown the component after some time - simulate an error during shutdown
		return NewShutdownErroringComponent(100 * time.Millisecond), nil
	}

	fatals := 0
	onError := func(err error) component.ErrorHandlingResult {
		fatals++
		require.ErrorIs(t, err, ErrFatal)
		if fatals < 3 { //restart component after first and second error
			return component.ErrorHandlingRestart
		} else { //stop component after third error
			return component.ErrorHandlingStop
		}
	}

	//irrelevant what context we use - test won't be using it
	err := component.RunComponent(context.Background(), componentFactory, onError)
	require.ErrorIs(t, err, ErrFatal)
	require.Equal(t, 3, fatals)
}

func TestRunComponentConcurrentError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	componentFactory := func() (component.Component, error) {
		return NewConcurrentErroringComponent(100 * time.Millisecond), nil
	}

	fatals := 0
	onError := func(err error) component.ErrorHandlingResult {
		fatals++
		require.ErrorIs(t, err, ErrFatal)
		if fatals < 2 {
			return component.ErrorHandlingRestart
		} else {
			return component.ErrorHandlingStop
		}
	}

	err := component.RunComponent(ctx, componentFactory, onError)
	require.ErrorIs(t, err, ErrFatal)
	require.Equal(t, 2, fatals)
}

func TestRunComponentNoError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	componentFactory := func() (component.Component, error) {
		return NewNonErroringComponent(100 * time.Millisecond), nil
	}

	onError := func(err error) component.ErrorHandlingResult {
		require.Fail(t, "error handler should not have been called")
		return component.ErrorHandlingStop
	}

	err := component.RunComponent(ctx, componentFactory, onError)
	require.NoError(t, err)
}

func TestRunComponentCancel(t *testing.T) {
	componentFactory := func() (component.Component, error) {
		return NewNonErroringComponent(math.MaxInt64), nil
	}

	onError := func(err error) component.ErrorHandlingResult {
		require.Fail(t, "error handler should not have been called")
		return component.ErrorHandlingStop
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := component.RunComponent(ctx, componentFactory, onError)
	require.ErrorIs(t, err, ctx.Err())
}

func TestRunComponentFactoryError(t *testing.T) {
	componentFactory := func() (component.Component, error) {
		return nil, ErrCouldNotCreateComponent
	}

	onError := func(err error) component.ErrorHandlingResult {
		require.Fail(t, "error handler should not have been called")
		return component.ErrorHandlingStop
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := component.RunComponent(ctx, componentFactory, onError)
	require.ErrorIs(t, err, ErrCouldNotCreateComponent)
}
