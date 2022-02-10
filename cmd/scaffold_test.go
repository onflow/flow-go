package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

type testReadyDone struct {
	name    string
	readyFn func(string) <-chan struct{}
	doneFn  func(string) <-chan struct{}
}

func (n *testReadyDone) Ready() <-chan struct{} {
	return n.readyFn(n.name)
}

func (n *testReadyDone) Done() <-chan struct{} {
	return n.doneFn(n.name)
}

type testComponent struct {
	*testReadyDone
	startFn func(irrecoverable.SignalerContext, string)
	started chan struct{}
}

func (n *testComponent) Start(ctx irrecoverable.SignalerContext) {
	defer close(n.started)
	n.startFn(ctx, n.name)
}

func (n *testComponent) Ready() <-chan struct{} {
	<-n.started
	return n.readyFn(n.name)
}

// Test the components are started in the correct order, and are run serially
func TestComponentsRunSerially(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	nb := FlowNode("scaffold test")
	nb.componentBuilder = component.NewComponentManagerBuilder()

	logger := testLog{}

	readyFn := func(name string) <-chan struct{} {
		ready := make(chan struct{})
		defer close(ready)
		logger.Logf("%s ready", name)
		return ready
	}
	doneFn := func(name string) <-chan struct{} {
		done := make(chan struct{})
		defer close(done)
		logger.Logf("%s done", name)
		return done
	}
	startFn := func(ctx irrecoverable.SignalerContext, name string) {
		// add delay to test components are run serially
		time.Sleep(5 * time.Millisecond)
		logger.Logf("%s started", name)
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
		return &testComponent{
			testReadyDone: &testReadyDone{
				name:    name2,
				readyFn: readyFn,
				doneFn:  doneFn,
			},
			startFn: startFn,
			started: make(chan struct{}),
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
