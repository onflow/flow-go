package cmd

import (
	"context"
	"os"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
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
	sigCtx, _ := util.WithSignal(ctx, os.Interrupt, syscall.SIGTERM)
	if err := util.WaitError(sigCtx, errChan, nil); err != nil {
		node.Logger.Fatal().Err(err).Msg("unhandled irrecoverable error")
	}

	node.Logger.Info().Msgf("%s node shutting down", node.NodeRole)
	cancel()

	if err := util.WaitError(sigCtx, errChan, node.Done()); err != nil {
		if err == sigCtx.Err() {
			node.Logger.Fatal().Msg("node shutdown aborted")
		}
		node.Logger.Fatal().Err(err).Msg("unhandled irrecoverable error during shutdown")
	}

	node.Logger.Info().Msgf("%s node shutdown complete", node.NodeRole)
	os.Exit(0)
}
