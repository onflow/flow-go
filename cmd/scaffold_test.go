package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestLoadSecretsEncryptionKey checks that the key file is read correctly if it exists
// and returns the expected sentinel error if it does not exist.
func TestLoadSecretsEncryptionKey(t *testing.T) {
	myID := unittest.IdentifierFixture()

	unittest.RunWithTempDir(t, func(dir string) {
		path := filepath.Join(dir, fmt.Sprintf(bootstrap.PathSecretsEncryptionKey, myID))

		t.Run("should return ErrNotExist if file doesn't exist", func(t *testing.T) {
			require.NoFileExists(t, path)
			_, err := loadSecretsEncryptionKey(dir, myID)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, os.ErrNotExist))
		})

		t.Run("should return key and no error if file exists", func(t *testing.T) {
			err := os.MkdirAll(filepath.Join(dir, bootstrap.DirPrivateRoot, fmt.Sprintf("private-node-info_%v", myID)), 0700)
			require.NoError(t, err)
			key, err := utils.GenerateSecretsDBEncryptionKey()
			require.NoError(t, err)
			err = ioutil.WriteFile(path, key, 0700)
			require.NoError(t, err)

			data, err := loadSecretsEncryptionKey(dir, myID)
			assert.NoError(t, err)
			assert.Equal(t, key, data)
		})
	})
}

// Test the components are started in the correct order, and are run serially
func TestComponentsRunSerially(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	nb := FlowNode("scaffold test")
	nb.componentBuilder = component.NewComponentManagerBuilder()

	logger := &testLog{}

	name1 := "component 1"
	nb.Component(name1, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name1)
		return newTestReadyDone(logger, name1), nil
	})

	name2 := "component 2"
	nb.Component(name2, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name2)
		c := newTestComponent(logger, name2)
		c.startFn = func(ctx irrecoverable.SignalerContext, name string) {
			// add delay to test components are run serially
			time.Sleep(5 * time.Millisecond)
		}
		return c, nil
	})

	name3 := "component 3"
	nb.Component(name3, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name3)
		return newTestReadyDone(logger, name3), nil
	})

	err := nb.handleComponents()
	assert.NoError(t, err)

	cm := nb.componentBuilder.Build()

	cm.Start(signalerCtx)
	<-cm.Ready()
	cancel()
	<-cm.Done()

	logs := logger.logs

	assert.Len(t, logs, 10)

	// components are initialized in a specific order, so check that the order is correct
	startLogs := logs[:len(logs)-3]
	assert.Equal(t, []string{
		"component 1 initialized",
		"component 1 ready",
		"component 2 initialized",
		"component 2 started",
		"component 2 ready",
		"component 3 initialized",
		"component 3 ready",
	}, startLogs)

	// components are stopped via context cancellation, so the specific order is random
	doneLogs := logs[len(logs)-3:]
	assert.ElementsMatch(t, []string{
		"component 1 done",
		"component 2 done",
		"component 3 done",
	}, doneLogs)
}

func TestPostShutdown(t *testing.T) {
	nb := FlowNode("scaffold test")

	logger := testLog{}

	err1 := errors.New("error 1")
	err3 := errors.New("error 3")
	errExpected := multierror.Append(&multierror.Error{}, err1, err3)
	nb.
		ShutdownFunc(func() error {
			logger.Log("shutdown 1")
			return err1
		}).
		ShutdownFunc(func() error {
			logger.Log("shutdown 2")
			return nil
		}).
		ShutdownFunc(func() error {
			logger.Log("shutdown 3")
			return err3
		})

	err := nb.postShutdown()
	assert.EqualError(t, err, errExpected.Error())

	logs := logger.logs
	assert.Len(t, logs, 3)
	assert.Equal(t, []string{
		"shutdown 1",
		"shutdown 2",
		"shutdown 3",
	}, logs)
}

func TestOverrideComponent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	nb := FlowNode("scaffold test")
	nb.componentBuilder = component.NewComponentManagerBuilder()

	logger := testLog{}

	readyFn := func(name string) {
		logger.Logf("%s ready", name)
	}
	doneFn := func(name string) {
		logger.Logf("%s done", name)
	}

	name1 := "component 1"
	nb.Component(name1, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name1)
		return &testReadyDone{
			name:    name1,
			readyFn: readyFn,
			doneFn:  doneFn,
		}, nil
	})

	name2 := "component 2"
	nb.Component(name2, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name2)
		return &testReadyDone{
			name:    name2,
			readyFn: readyFn,
			doneFn:  doneFn,
		}, nil
	})

	name3 := "component 3"
	nb.Component(name3, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name3)
		return &testReadyDone{
			name:    name3,
			readyFn: readyFn,
			doneFn:  doneFn,
		}, nil
	})

	// Overrides second component
	nb.OverrideComponent(name2, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s overridden", name2)
		return &testReadyDone{
			name:    name2,
			readyFn: readyFn,
			doneFn:  doneFn,
		}, nil
	})

	err := nb.handleComponents()
	assert.NoError(t, err)

	cm := nb.componentBuilder.Build()

	cm.Start(signalerCtx)

	<-cm.Ready()

	logs := logger.logs

	assert.Len(t, logs, 6)

	// components are initialized in a specific order, so check that the order is correct
	assert.Equal(t, []string{
		"component 1 initialized",
		"component 1 ready",
		"component 2 overridden", // overridden version of 2 should be initialized.
		"component 2 ready",
		"component 3 initialized",
		"component 3 ready",
	}, logs)

	cancel()
	<-cm.Done()
}

type testComponentDefinition struct {
	name         string
	factory      ReadyDoneFactory
	errorHandler component.OnError
}

func runRestartableTest(t *testing.T, components []testComponentDefinition, expectedErr error, shutdown <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	go func() {
		select {
		case <-ctx.Done():
			return
		case err := <-errChan:
			if expectedErr == nil {
				assert.NoError(t, err, "unexpected unhandled irrecoverable error")
			} else {
				assert.ErrorIs(t, err, expectedErr)
			}
		}
	}()

	nb := FlowNode("scaffold test")
	nb.componentBuilder = component.NewComponentManagerBuilder()

	for _, c := range components {
		if c.errorHandler == nil {
			nb.Component(c.name, c.factory)
		} else {
			nb.RestartableComponent(c.name, c.factory, c.errorHandler)
		}
	}

	err := nb.handleComponents()
	assert.NoError(t, err)

	cm := nb.componentBuilder.Build()

	cm.Start(signalerCtx)

	<-shutdown
	cancel()

	<-cm.Done()
}

func TestRestartableRestartsSuccessfully(t *testing.T) {

	logger := &testLog{}
	shutdown := make(chan struct{})

	name := "component 1"
	err := fmt.Errorf("%s error", name)

	starts := 0
	factory := func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name)
		c := newTestComponent(logger, name)
		c.startFn = func(signalCtx irrecoverable.SignalerContext, name string) {
			go func() {
				<-c.Ready()
				starts++
				if starts == 1 {
					signalCtx.Throw(err)
				}
				close(shutdown)
			}()
		}
		return c, nil
	}

	runRestartableTest(t, []testComponentDefinition{
		{
			name:         name,
			factory:      factory,
			errorHandler: testErrorHandler(logger, err),
		},
	}, nil, shutdown)

	assert.Equal(t, []string{
		"component 1 initialized",
		"component 1 started",
		"component 1 ready",
		"component 1 done",
		"handled error: component 1 error",
		"component 1 initialized",
		"component 1 started",
		"component 1 ready",
		"component 1 done",
	}, logger.logs)
}

func TestRestartableStopsSuccessfully(t *testing.T) {
	logger := &testLog{}
	shutdown := make(chan struct{})

	name := "component 1"
	err := fmt.Errorf("%s error", name)
	unexpectedErr := fmt.Errorf("%s unexpected error", name)

	starts := 0
	factory := func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name)
		c := newTestComponent(logger, name)
		c.startFn = func(signalCtx irrecoverable.SignalerContext, name string) {
			go func() {
				<-c.Ready()
				starts++
				if starts < 2 {
					signalCtx.Throw(err)
				}
				if starts == 2 {
					defer close(shutdown)
					signalCtx.Throw(unexpectedErr)
				}
			}()
		}
		return c, nil
	}

	runRestartableTest(t, []testComponentDefinition{
		{
			name:         name,
			factory:      factory,
			errorHandler: testErrorHandler(logger, err),
		},
	}, unexpectedErr, shutdown)

	assert.Equal(t, []string{
		"component 1 initialized",
		"component 1 started",
		"component 1 ready",
		"component 1 done",
		"handled error: component 1 error",
		"component 1 initialized",
		"component 1 started",
		"component 1 ready",
		"component 1 done",
		"handled unexpected error: component 1 unexpected error",
	}, logger.logs)
}

func TestRestartableWithMultipleComponents(t *testing.T) {
	logger := &testLog{}
	shutdown := make(chan struct{})

	// synchronization is needed since RestartableComponents are non-blocking
	readyComponents := sync.WaitGroup{}
	readyComponents.Add(3)

	c1 := func() testComponentDefinition {
		name := "component 1"
		factory := func(node *NodeConfig) (module.ReadyDoneAware, error) {
			logger.Logf("%s initialized", name)
			c := newTestReadyDone(logger, name)
			c.readyFn = func(name string) {
				// delay to demonstrate that components are started serially
				time.Sleep(5 * time.Millisecond)
				readyComponents.Done()
			}
			return c, nil
		}

		return testComponentDefinition{
			name:    name,
			factory: factory,
		}
	}

	c2Initialized := make(chan struct{})
	c2 := func() testComponentDefinition {
		name := "component 2"
		err := fmt.Errorf("%s error", name)
		factory := func(node *NodeConfig) (module.ReadyDoneAware, error) {
			defer close(c2Initialized)
			logger.Logf("%s initialized", name)
			c := newTestComponent(logger, name)
			c.startFn = func(ctx irrecoverable.SignalerContext, name string) {
				// delay to demonstrate the RestartableComponent startup is non-blocking
				time.Sleep(5 * time.Millisecond)
			}
			c.readyFn = func(name string) {
				readyComponents.Done()
			}
			return c, nil
		}

		return testComponentDefinition{
			name:         name,
			factory:      factory,
			errorHandler: testErrorHandler(logger, err),
		}
	}

	c3 := func() testComponentDefinition {
		name := "component 3"
		err := fmt.Errorf("%s error", name)
		starts := 0
		factory := func(node *NodeConfig) (module.ReadyDoneAware, error) {
			logger.Logf("%s initialized", name)
			c := newTestComponent(logger, name)
			c.startFn = func(signalCtx irrecoverable.SignalerContext, name string) {
				go func() {
					<-c.Ready()
					starts++
					if starts == 1 {
						signalCtx.Throw(err)
					}
					<-c2Initialized // can't use ready since it may not be initialized yet
					readyComponents.Done()
				}()
			}
			return c, nil
		}

		return testComponentDefinition{
			name:         name,
			factory:      factory,
			errorHandler: testErrorHandler(logger, err),
		}
	}

	go func() {
		readyComponents.Wait()
		close(shutdown)
	}()

	runRestartableTest(t, []testComponentDefinition{c1(), c2(), c3()}, nil, shutdown)

	logs := logger.logs

	// make sure component 1 is started and ready before any other components start
	assert.Equal(t, []string{
		"component 1 initialized",
		"component 1 ready",
	}, logs[:2])

	// now split logs by component, and verify we got the right messages/order
	component1 := []string{}
	component2 := []string{}
	component3 := []string{}
	unexpected := []string{}
	for _, l := range logs {
		switch {
		case strings.Contains(l, "component 1"):
			component1 = append(component1, l)
		case strings.Contains(l, "component 2"):
			component2 = append(component2, l)
		case strings.Contains(l, "component 3"):
			component3 = append(component3, l)
		default:
			unexpected = append(unexpected, l)
		}
	}

	// no unexpected logs
	assert.Len(t, unexpected, 0)

	assert.Equal(t, []string{
		"component 1 initialized",
		"component 1 ready",
		"component 1 done",
	}, component1)

	assert.Equal(t, []string{
		"component 2 initialized",
		"component 2 started",
		"component 2 ready",
		"component 2 done",
	}, component2)

	assert.Equal(t, []string{
		"component 3 initialized",
		"component 3 started",
		"component 3 ready",
		"component 3 done",
		"handled error: component 3 error",
		"component 3 initialized",
		"component 3 started",
		"component 3 ready",
		"component 3 done",
	}, component3)

	// components are stopped via context cancellation, so the specific order is random
	doneLogs := logs[len(logs)-3:]
	assert.ElementsMatch(t, []string{
		"component 1 done",
		"component 2 done",
		"component 3 done",
	}, doneLogs)
}

func testErrorHandler(logger *testLog, expected error) component.OnError {
	return func(err error) component.ErrorHandlingResult {
		if errors.Is(err, expected) {
			logger.Logf("handled error: %s", err)
			return component.ErrorHandlingRestart
		}
		logger.Logf("handled unexpected error: %s", err)
		return component.ErrorHandlingStop
	}
}

func newTestReadyDone(logger *testLog, name string) *testReadyDone {
	return &testReadyDone{
		name:    name,
		logger:  logger,
		readyFn: func(string) {},
		doneFn:  func(string) {},
		ready:   make(chan struct{}),
		done:    make(chan struct{}),
	}
}

type testReadyDone struct {
	name   string
	logger *testLog

	readyFn func(string)
	doneFn  func(string)

	ready chan struct{}
	done  chan struct{}

	started bool
	stopped bool
	mu      sync.Mutex
}

func (c *testReadyDone) Ready() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		c.started = true
		go func() {
			c.readyFn(c.name)

			c.logger.Logf("%s ready", c.name)
			close(c.ready)
		}()
	}

	return c.ready
}

func (c *testReadyDone) Done() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		c.stopped = true
		go func() {
			c.doneFn(c.name)

			c.logger.Logf("%s done", c.name)
			close(c.done)
		}()
	}

	return c.done
}

func newTestComponent(logger *testLog, name string) *testComponent {
	return &testComponent{
		name:    name,
		logger:  logger,
		readyFn: func(string) {},
		doneFn:  func(string) {},
		startFn: func(irrecoverable.SignalerContext, string) {},
		ready:   make(chan struct{}),
		done:    make(chan struct{}),
	}
}

type testComponent struct {
	name   string
	logger *testLog

	readyFn func(string)
	doneFn  func(string)
	startFn func(irrecoverable.SignalerContext, string)

	ready chan struct{}
	done  chan struct{}
}

func (c *testComponent) Start(ctx irrecoverable.SignalerContext) {
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

func (c *testComponent) Ready() <-chan struct{} {
	return c.ready
}

func (c *testComponent) Done() <-chan struct{} {
	return c.done
}
