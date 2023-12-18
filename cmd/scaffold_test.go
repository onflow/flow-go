package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	gcemd "cloud.google.com/go/compute/metadata"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/profiler"
	"github.com/onflow/flow-go/network/p2p/builder"
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
			err = os.WriteFile(path, key, 0700)
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
		return newMockReadyDone(logger, name1), nil
	})

	name2 := "component 2"
	nb.Component(name2, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name2)
		c := newMockComponent(logger, name2)
		c.startFn = func(ctx irrecoverable.SignalerContext, name string) {
			// add delay to test components are run serially
			time.Sleep(5 * time.Millisecond)
		}
		return c, nil
	})

	name3 := "component 3"
	nb.Component(name3, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name3)
		return newMockReadyDone(logger, name3), nil
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

	logger := &testLog{}

	name1 := "component 1"
	nb.Component(name1, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name1)
		return newMockReadyDone(logger, name1), nil
	})

	name2 := "component 2"
	nb.Component(name2, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name2)
		return newMockReadyDone(logger, name2), nil
	})

	name3 := "component 3"
	nb.Component(name3, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name3)
		return newMockReadyDone(logger, name3), nil
	})

	// Overrides second component
	nb.OverrideComponent(name2, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s overridden", name2)
		return newMockReadyDone(logger, name2), nil
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

func TestOverrideModules(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	nb := FlowNode("scaffold test")
	nb.componentBuilder = component.NewComponentManagerBuilder()

	logger := &testLog{}

	name1 := "module 1"
	nb.Module(name1, func(nodeConfig *NodeConfig) error {
		logger.Logf("%s initialized", name1)
		return nil
	})

	name2 := "module 2"
	nb.Module(name2, func(nodeConfig *NodeConfig) error {
		logger.Logf("%s initialized", name2)
		return nil
	})

	name3 := "module 3"
	nb.Module(name3, func(nodeConfig *NodeConfig) error {
		logger.Logf("%s initialized", name3)
		return nil
	})

	// Overrides second module
	nb.OverrideModule(name2, func(nodeConfig *NodeConfig) error {
		logger.Logf("%s overridden", name2)
		return nil
	})

	err := nb.handleModules()
	assert.NoError(t, err)

	cm := nb.componentBuilder.Build()
	require.NoError(t, err)

	cm.Start(signalerCtx)

	<-cm.Ready()

	logs := logger.logs

	assert.Len(t, logs, 3)

	// components are initialized in a specific order, so check that the order is correct
	assert.Equal(t, []string{
		"module 1 initialized",
		"module 2 overridden", // overridden version of 2 should be initialized.
		"module 3 initialized",
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
		c := newMockComponent(logger, name)
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
		c := newMockComponent(logger, name)
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
			c := newMockReadyDone(logger, name)
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
			c := newMockComponent(logger, name)
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
			c := newMockComponent(logger, name)
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

// TestDependableComponentWaitForDependencies tests that dependable components are started after
// their dependencies are ready
// In this test:
// * Components 1 & 2 are DependableComponents
// * Component 3 is a normal Component
// * 1 depends on 3
// * 2 depends on 1
// * Start order should be 3, 1, 2
// run test 10 times to ensure order is consistent
func TestDependableComponentWaitForDependencies(t *testing.T) {
	for i := 0; i < 10; i++ {
		testDependableComponentWaitForDependencies(t)
	}
}

func testDependableComponentWaitForDependencies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	nb := FlowNode("scaffold test")
	nb.componentBuilder = component.NewComponentManagerBuilder()

	logger := &testLog{}

	component1Dependable := module.NewProxiedReadyDoneAware()
	component3Dependable := module.NewProxiedReadyDoneAware()

	name1 := "component 1"
	nb.DependableComponent(name1, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name1)
		c := newMockComponent(logger, name1)
		component1Dependable.Init(c)
		return c, nil
	}, &DependencyList{[]module.ReadyDoneAware{component3Dependable}})

	name2 := "component 2"
	nb.DependableComponent(name2, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name2)
		return newMockComponent(logger, name2), nil
	}, &DependencyList{[]module.ReadyDoneAware{component1Dependable}})

	name3 := "component 3"
	nb.Component(name3, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		logger.Logf("%s initialized", name3)
		c := newMockComponent(logger, name3)
		c.startFn = func(ctx irrecoverable.SignalerContext, name string) {
			// add delay to test components are run serially
			time.Sleep(5 * time.Millisecond)
		}
		component3Dependable.Init(c)
		return c, nil
	})

	err := nb.handleComponents()
	require.NoError(t, err)

	cm := nb.componentBuilder.Build()

	cm.Start(signalerCtx)
	<-cm.Ready()

	cancel()
	<-cm.Done()

	logs := logger.logs

	assert.Len(t, logs, 12)

	// components are initialized in a specific order, so check that the order is correct
	startLogs := logs[:len(logs)-3]
	assert.Equal(t, []string{
		"component 3 initialized",
		"component 3 started",
		"component 3 ready",
		"component 1 initialized",
		"component 1 started",
		"component 1 ready",
		"component 2 initialized",
		"component 2 started",
		"component 2 ready",
	}, startLogs)

	// components are stopped via context cancellation, so the specific order is random
	doneLogs := logs[len(logs)-3:]
	assert.ElementsMatch(t, []string{
		"component 1 done",
		"component 2 done",
		"component 3 done",
	}, doneLogs)
}

func TestCreateUploader(t *testing.T) {
	t.Parallel()
	t.Run("create uploader", func(t *testing.T) {
		t.Parallel()
		nb := FlowNode("scaffold_uploader")
		mockHttp := &http.Client{
			Transport: &mockRoundTripper{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					switch req.URL.Path {
					case "/computeMetadata/v1/project/project-id":
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString("test-project-id")),
						}, nil
					case "/computeMetadata/v1/instance/id":
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString("test-instance-id")),
						}, nil
					default:
						return nil, fmt.Errorf("unexpected request: %s", req.URL.Path)
					}
				},
			},
		}

		testClient := gcemd.NewClient(mockHttp)
		uploader, err := nb.createGCEProfileUploader(
			testClient,

			option.WithoutAuthentication(),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		)
		require.NoError(t, err)
		require.NotNil(t, uploader)

		uploaderImpl, ok := uploader.(*profiler.UploaderImpl)
		require.True(t, ok)

		assert.Equal(t, "test-project-id", uploaderImpl.Deployment.ProjectId)
		assert.Equal(t, "unknown-scaffold_uploader", uploaderImpl.Deployment.Target)
		assert.Equal(t, "test-instance-id", uploaderImpl.Deployment.Labels["instance"])
		assert.Equal(t, "undefined-undefined", uploaderImpl.Deployment.Labels["version"])
	})
}

type mockRoundTripper struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

// TestDhtSystemActivationStatus tests that the DHT system activation status is correctly
// determined based on the role string.
// This test is not exhaustive, but should cover the most common cases.
func TestDhtSystemActivationStatus(t *testing.T) {
	tests := []struct {
		name      string
		roleStr   string
		expected  p2pbuilder.DhtSystemActivation
		expectErr bool
	}{
		{
			name:      "ghost role returns disabled",
			roleStr:   "ghost",
			expected:  p2pbuilder.DhtSystemDisabled,
			expectErr: false,
		},
		{
			name:      "access role returns enabled",
			roleStr:   "access",
			expected:  p2pbuilder.DhtSystemEnabled,
			expectErr: false,
		},
		{
			name:      "execution role returns enabled",
			roleStr:   "execution",
			expected:  p2pbuilder.DhtSystemEnabled,
			expectErr: false,
		},
		{
			name:      "collection role returns disabled",
			roleStr:   "collection",
			expected:  p2pbuilder.DhtSystemDisabled,
			expectErr: false,
		},
		{
			name:      "consensus role returns disabled",
			roleStr:   "consensus",
			expected:  p2pbuilder.DhtSystemDisabled,
			expectErr: false,
		},
		{
			name:      "verification nodes return disabled",
			roleStr:   "verification",
			expected:  p2pbuilder.DhtSystemDisabled,
			expectErr: false,
		},
		{
			name:      "invalid role returns error",
			roleStr:   "invalidRole",
			expected:  p2pbuilder.DhtSystemDisabled,
			expectErr: true,
		}, // Add more test cases for other roles, if needed.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DhtSystemActivationStatus(tt.roleStr)
			require.Equal(t, tt.expectErr, err != nil, "unexpected error status")
			require.Equal(t, tt.expected, result, "unexpected activation status")
		})
	}
}
