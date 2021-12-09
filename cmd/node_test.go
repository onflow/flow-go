package cmd

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

func TestRunShutsDownCleanly(t *testing.T) {
	testLogger := testLog{}
	logger := zerolog.New(os.Stdout)
	nodeConfig := &NodeConfig{BaseConfig: BaseConfig{NodeRole: "nodetest"}}
	postShutdown := func() error {
		testLogger.Log("running cleanup")
		return nil
	}
	fatalHandler := func(err error) {
		testLogger.Logf("received fatal error: %s", err)
	}

	t.Run("Run shuts down gracefully", func(t *testing.T) {
		testLogger.Reset()
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				testLogger.Log("worker starting up")
				ready()
				testLogger.Log("worker startup complete")

				<-ctx.Done()
				testLogger.Log("worker shutting down")
				testLogger.Log("worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, postShutdown, fatalHandler)

		finished := make(chan struct{})
		go func() {
			node.Run()
			close(finished)
		}()

		<-node.Ready()

		syscall.Kill(syscall.Getpid(), syscall.SIGINT)

		<-finished

		assert.Equal(t, []string{
			"worker starting up",
			"worker startup complete",
			"worker shutting down",
			"worker shutdown complete",
			"running cleanup",
		}, testLogger.logs)
	})

	t.Run("Run encounters error during postShutdown", func(t *testing.T) {
		testLogger.Reset()
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				testLogger.Log("worker starting up")
				ready()
				testLogger.Log("worker startup complete")

				<-ctx.Done()
				testLogger.Log("worker shutting down")
				testLogger.Log("worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, func() error {
			testLogger.Log("error during post shutdown")
			return errors.New("error during post shutdown")
		}, fatalHandler)

		finished := make(chan struct{})
		go func() {
			node.Run()
			close(finished)
		}()

		<-node.Ready()

		syscall.Kill(syscall.Getpid(), syscall.SIGINT)

		<-finished

		assert.Equal(t, []string{
			"worker starting up",
			"worker startup complete",
			"worker shutting down",
			"worker shutdown complete",
			"error during post shutdown",
		}, testLogger.logs)
	})

	t.Run("Run encounters irrecoverable error during startup", func(t *testing.T) {
		testLogger.Reset()
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				testLogger.Log("worker starting up")

				// throw an irrecoverable error
				ctx.Throw(errors.New("worker startup error"))

				ready()
				testLogger.Log("worker startup complete")

				<-ctx.Done()
				testLogger.Log("worker shutting down")
				testLogger.Log("worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, postShutdown, fatalHandler)

		finished := make(chan struct{})
		go func() {
			node.Run()
			close(finished)
		}()

		<-finished

		assert.Equal(t, []string{
			"worker starting up",
			"received fatal error: worker startup error",
		}, testLogger.logs)
	})

	t.Run("Run encounters irrecoverable error during runtime", func(t *testing.T) {
		testLogger.Reset()
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				testLogger.Log("worker starting up")
				ready()
				testLogger.Log("worker startup complete")

				// throw an irrecoverable error
				ctx.Throw(errors.New("worker runtime error"))

				<-ctx.Done()
				testLogger.Log("worker shutting down")
				testLogger.Log("worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, postShutdown, func(err error) {
			testLogger.Log("received fatal error: " + err.Error())
		})

		finished := make(chan struct{})
		go func() {
			node.Run()
			close(finished)
		}()

		<-finished

		assert.Equal(t, []string{
			"worker starting up",
			"worker startup complete",
			"received fatal error: worker runtime error",
		}, testLogger.logs)
	})

	t.Run("Run encounters irrecoverable error during shutdown", func(t *testing.T) {
		testLogger.Reset()
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				testLogger.Log("worker starting up")
				ready()
				testLogger.Log("worker startup complete")

				<-ctx.Done()
				testLogger.Log("worker shutting down")

				// throw an irrecoverable error
				ctx.Throw(errors.New("worker shutdown error"))

				testLogger.Log("worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, postShutdown, fatalHandler)

		finished := make(chan struct{})
		go func() {
			node.Run()
			close(finished)
		}()

		<-node.Ready()

		syscall.Kill(syscall.Getpid(), syscall.SIGINT)

		<-finished

		assert.Equal(t, []string{
			"worker starting up",
			"worker startup complete",
			"worker shutting down",
			"received fatal error: worker shutdown error",
		}, testLogger.logs)
	})
}
