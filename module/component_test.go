package module

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

var ErrFatal = errors.New("fatal")

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

		close(c.ready)

		// do work...
		<-ctx.Done()
	}()
}
func (c *StartupErroringComponent) Ready() <-chan struct{} {
	return c.ready
}

func (c *StartupErroringComponent) Done() <-chan struct{} {
	return c.done
}

type StartErroringComponent struct {
	ready chan struct{}
	done  chan struct{}
}

func NewStartErroringComponent() *StartErroringComponent {
	return &StartErroringComponent{
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
}

func (c *StartErroringComponent) Start(ctx irrecoverable.SignalerContext) {
	defer close(c.done)

	// throw fatal error synchronously during startup
	ctx.Throw(ErrFatal)
}
func (c *StartErroringComponent) Ready() <-chan struct{} {
	return c.ready
}

func (c *StartErroringComponent) Done() <-chan struct{} {
	return c.done
}

type ShutdownErroringComponent struct {
	ready   sync.WaitGroup
	done    sync.WaitGroup
	started chan struct{}
}

func NewShutdownErroringComponent() *ShutdownErroringComponent {
	return &ShutdownErroringComponent{
		started: make(chan struct{}),
	}
}

func (c *ShutdownErroringComponent) Start(ctx irrecoverable.SignalerContext) {
	c.ready.Add(2)
	c.done.Add(2)

	go func() {
		c.ready.Done()
		defer c.done.Done()

		// to some work...
		time.Sleep(100 * time.Millisecond)

		// encounter fatal error
		ctx.Throw(ErrFatal)
	}()

	go func() {
		c.ready.Done()
		defer c.done.Done()

		// wait for shutdown signal
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
	ready   sync.WaitGroup
	done    sync.WaitGroup
	started chan struct{}
}

func NewConcurrentErroringComponent() *ConcurrentErroringComponent {
	return &ConcurrentErroringComponent{
		started: make(chan struct{}),
	}
}

func (c *ConcurrentErroringComponent) Start(ctx irrecoverable.SignalerContext) {
	c.ready.Add(2)
	c.done.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			c.ready.Done()
			defer c.done.Done()

			// to some work...
			time.Sleep(100 * time.Millisecond)

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

func TestRunComponentStartError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	componentFactory := func() (Component, error) {
		return NewStartErroringComponent(), nil
	}

	onError := func(err error, triggerRestart func()) {
		require.FailNow(t, "error handler should never be called")
	}

	err := RunComponent(ctx, componentFactory, onError)
	require.ErrorIs(t, err, ErrFatal)
}
func TestRunComponentStartupError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	componentFactory := func() (Component, error) {
		return NewStartupErroringComponent(), nil
	}

	called := false
	onError := func(err error, triggerRestart func()) {
		called = true
		require.ErrorIs(t, err, ErrFatal)
		cancel()
	}

	err := RunComponent(ctx, componentFactory, onError)
	require.ErrorIs(t, err, context.Canceled)
	require.True(t, called)
}

func TestRunComponentShutdownError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	componentFactory := func() (Component, error) {
		return NewShutdownErroringComponent(), nil
	}

	fatals := 0
	onError := func(err error, triggerRestart func()) {
		fatals++
		require.ErrorIs(t, err, ErrFatal)
		if fatals < 2 {
			triggerRestart()
		} else {
			cancel()
		}
	}

	err := RunComponent(ctx, componentFactory, onError)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 2, fatals)
}

func TestRunComponentConcurrentError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	componentFactory := func() (Component, error) {
		return NewConcurrentErroringComponent(), nil
	}

	fatals := 0
	onError := func(err error, triggerRestart func()) {
		fatals++
		require.ErrorIs(t, err, ErrFatal)
		if fatals < 2 {
			triggerRestart()
		} else {
			cancel()
		}
	}

	err := RunComponent(ctx, componentFactory, onError)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 2, fatals)
}

// TestAllReady tests that AllReady closes its returned Ready channel only once
// all input ReadyDone instances close their Ready channel.
func TestAllReady(t *testing.T) {
	cases := []int{0, 1, 100}
	for _, n := range cases {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			testAllReady(n, t)
		})
	}
}

// TestAllDone tests that AllDone closes its returned Done channel only once
// all input ReadyDone instances close their Done channel.
func TestAllDone(t *testing.T) {
	cases := []int{0, 1, 100}
	for _, n := range cases {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			testAllDone(n, t)
		})
	}
}

func testAllDone(n int, t *testing.T) {

	components := make([]realmodule.ReadyDoneAware, n)
	for i := 0; i < n; i++ {
		components[i] = new(module.ReadyDoneAware)
		unittest.ReadyDoneify(components[i])
	}

	unittest.AssertClosesBefore(t, lifecycle.AllReady(components...), time.Second)

	for _, component := range components {
		mock := component.(*module.ReadyDoneAware)
		mock.AssertCalled(t, "Ready")
		mock.AssertNotCalled(t, "Done")
	}
}

func testAllReady(n int, t *testing.T) {

	components := make([]realmodule.ReadyDoneAware, n)
	for i := 0; i < n; i++ {
		components[i] = new(module.ReadyDoneAware)
		unittest.ReadyDoneify(components[i])
	}

	unittest.AssertClosesBefore(t, module.AllDone(components...), time.Second)

	for _, component := range components {
		mock := component.(*module.ReadyDoneAware)
		mock.AssertCalled(t, "Done")
		mock.AssertNotCalled(t, "Ready")
	}
}
