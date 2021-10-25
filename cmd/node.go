package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type Node interface {
	component.Component

	// Run initiates all common components (logger, database, protocol state etc.)
	// then starts each component. It also sets up a channel to gracefully shut
	// down each component if a SIGINT is received.
	Run()

	// ShutdownSignal returns a channel that is closed when shutdown has commenced.
	ShutdownSignal() <-chan struct{}
}

type FlowNodeImp struct {
	*component.ComponentManager
	NodeRole string
	Logger   zerolog.Logger
}

// Run calls Start() to start all the node modules and components. It also sets up a channel to gracefully shut
// down each component if a SIGINT is received. Until a SIGINT is received, Run will block.
// Since, Run is a blocking call it should only be used when running a node as it's own independent process.
// Any unhandled irrecoverable errors thrown in child components will bubble up to here and result in a fatal
// error
func (node *FlowNodeImp) Run() {

	// initialize signal catcher
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go node.Start(signalerCtx)

	go func() {
		select {
		case <-node.Ready():
			node.Logger.Info().Msgf("%s node startup complete", node.NodeRole)
		case <-ctx.Done():
		}
	}()

	// block till a SIGINT is received or a fatal error is encountered
	select {
	case <-sig:
		// address race condition caused by random selection when both select conditions
		// are met at the same time
		select {
		case err := <-errChan:
			node.Logger.Fatal().Err(err).Msg("unhandled irrecoverable error")
		default:
		}
	case err := <-errChan:
		node.Logger.Fatal().Err(err).Msg("unhandled irrecoverable error")
	}

	node.Logger.Info().Msgf("%s node shutting down", node.NodeRole)
	cancel()

	select {
	case <-node.Done():
	case err := <-errChan:
		node.Logger.Fatal().Err(err).Msg("unhandled irrecoverable error during shutdown")
	case <-sig:
		node.Logger.Fatal().Msg("node shutdown aborted")
	}

	node.Logger.Info().Msgf("%s node shutdown complete", node.NodeRole)
	os.Exit(0)
}
