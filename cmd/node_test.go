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
	var log []string
	logger := zerolog.New(os.Stdout)
	nodeConfig := &NodeConfig{BaseConfig: BaseConfig{NodeRole: "nodetest"}}
	postShutdown := func() error {
		log = append(log, "running cleanup")
		return nil
	}
	fatalHandler := func(err error, logger zerolog.Logger) {
		log = append(log, "received fatal error: "+err.Error())
	}

	t.Run("Run shuts down gracefully", func(t *testing.T) {
		log = []string{}
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				log = append(log, "worker starting up")
				ready()
				log = append(log, "worker startup complete")

				<-ctx.Done()
				log = append(log, "worker shutting down")
				log = append(log, "worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, postShutdown)

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
		}, log)
	})

	t.Run("Run encounters error during postShutdown", func(t *testing.T) {
		log = []string{}
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				log = append(log, "worker starting up")
				ready()
				log = append(log, "worker startup complete")

				<-ctx.Done()
				log = append(log, "worker shutting down")
				log = append(log, "worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, func() error {
			log = append(log, "error during post shutdown")
			return errors.New("error during post shutdown")
		})

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
		}, log)
	})

	t.Run("Run encounters irrecoverable error during startup", func(t *testing.T) {
		log = []string{}
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				log = append(log, "worker starting up")
				logger.Info().Msg("worker starting up")
				ctx.Throw(errors.New("worker startup error"))
				ready()
				log = append(log, "worker startup complete")

				<-ctx.Done()
				log = append(log, "worker shutting down")
				log = append(log, "worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, postShutdown)
		node.(*FlowNodeImp).fatalHandler = fatalHandler

		finished := make(chan struct{})
		go func() {
			node.Run()
			close(finished)
		}()

		<-finished

		assert.Equal(t, []string{
			"worker starting up",
			"received fatal error: worker startup error",
		}, log)
	})

	t.Run("Run encounters irrecoverable error during runtime", func(t *testing.T) {
		log = []string{}
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				log = append(log, "worker starting up")
				ready()
				log = append(log, "worker startup complete")

				ctx.Throw(errors.New("worker runtime error"))

				<-ctx.Done()
				log = append(log, "worker shutting down")
				log = append(log, "worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, postShutdown)
		node.(*FlowNodeImp).fatalHandler = func(err error, logger zerolog.Logger) {
			logger.Info().Msg("handling fatal error")
			log = append(log, "received fatal error: "+err.Error())
		}

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
		}, log)
	})

	t.Run("Run encounters irrecoverable error during shutdown", func(t *testing.T) {
		log = []string{}
		manager := component.NewComponentManagerBuilder().
			AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				log = append(log, "worker starting up")
				ready()
				log = append(log, "worker startup complete")

				<-ctx.Done()
				log = append(log, "worker shutting down")
				ctx.Throw(errors.New("worker shutdown error"))
				log = append(log, "worker shutdown complete")
			}).
			Build()
		node := NewNode(manager, nodeConfig, logger, postShutdown)
		node.(*FlowNodeImp).fatalHandler = fatalHandler

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
		}, log)
	})
}
