package cmd

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/module/irrecoverable"
)

type testLog struct {
	logs []string
	mux  sync.Mutex
}

// handle concurrent logging
func (l *testLog) Logf(msg string, args ...interface{}) {
	l.Log(fmt.Sprintf(msg, args...))
}

func (l *testLog) Log(msg string) {
	l.mux.Lock()
	defer l.mux.Unlock()

	l.logs = append(l.logs, msg)
}

func (l *testLog) Reset() {
	l.mux.Lock()
	defer l.mux.Unlock()

	l.logs = []string{}
}

func newMockReadyDone(logger *testLog, name string) *mockReadyDone {
	return &mockReadyDone{
		name:    name,
		logger:  logger,
		readyFn: func(string) {},
		doneFn:  func(string) {},
		ready:   make(chan struct{}),
		done:    make(chan struct{}),
	}
}

type mockReadyDone struct {
	name   string
	logger *testLog

	readyFn func(string)
	doneFn  func(string)

	ready chan struct{}
	done  chan struct{}

	startOnce sync.Once
	stopOnce  sync.Once
}

func (c *mockReadyDone) Ready() <-chan struct{} {
	c.startOnce.Do(func() {
		go func() {
			c.readyFn(c.name)

			c.logger.Logf("%s ready", c.name)
			close(c.ready)
		}()
	})

	return c.ready
}

func (c *mockReadyDone) Done() <-chan struct{} {
	c.stopOnce.Do(func() {
		go func() {
			c.doneFn(c.name)

			c.logger.Logf("%s done", c.name)
			close(c.done)
		}()
	})

	return c.done
}

func newMockComponent(logger *testLog, name string) *mockComponent {
	return &mockComponent{
		name:    name,
		logger:  logger,
		readyFn: func(string) {},
		doneFn:  func(string) {},
		startFn: func(irrecoverable.SignalerContext, string) {},
		ready:   make(chan struct{}),
		done:    make(chan struct{}),
	}
}

type mockComponent struct {
	name   string
	logger *testLog

	readyFn func(string)
	doneFn  func(string)
	startFn func(irrecoverable.SignalerContext, string)

	ready chan struct{}
	done  chan struct{}
}

func (c *mockComponent) Start(ctx irrecoverable.SignalerContext) {
	c.startFn(ctx, c.name)
	c.logger.Logf("%s started", c.name)

	go func() {
		c.readyFn(c.name)
		c.logger.Logf("%s ready", c.name)
		close(c.ready)
	}()

	go func() {
		<-ctx.Done()

		c.doneFn(c.name)
		c.logger.Logf("%s done", c.name)
		close(c.done)
	}()
}

func (c *mockComponent) Ready() <-chan struct{} {
	return c.ready
}

func (c *mockComponent) Done() <-chan struct{} {
	return c.done
}
