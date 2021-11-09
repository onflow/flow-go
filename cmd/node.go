package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
)

var _ component.Component = (*FlowNodeImp)(nil)

type Node interface {
	component.Component

	// Run initiates all common components (logger, database, protocol state etc.)
	// then starts each component. It also sets up a channel to gracefully shut
	// down each component if a SIGINT is received.
	Run()
}

type FlowNodeImp struct {
	*component.ComponentManager
	*NodeConfig
	Logger       zerolog.Logger
	postShutdown func()
}

// Run calls Start() to start all the node modules and components. It also sets up a channel to gracefully shut
// down each component if a SIGINT is received. Until a SIGINT is received, Run will block.
// Since, Run is a blocking call it should only be used when running a node as it's own independent process.
// Any unhandled irrecoverable errors thrown in child components will propagate up to here and result in a fatal
// error
func (node *FlowNodeImp) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go node.Start(signalerCtx)

	go func() {
		select {
		case <-node.Ready():
			node.Logger.Info().Msgf("%s node startup complete", node.BaseConfig.NodeRole)
		case <-ctx.Done():
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// block till a SIGINT is received or a fatal error is encountered
	sigCtx, _ := util.WithSignal(ctx, signalChan)
	if err := util.WaitError(sigCtx, errChan); err != nil {
		node.Logger.Fatal().Err(err).Msg("unhandled irrecoverable error")
	}

	node.Logger.Info().Msgf("%s node shutting down", node.BaseConfig.NodeRole)
	cancel()

	sigCtx, _ = util.WithSignal(context.Background(), signalChan)
	doneCtx, _ := util.WithDone(sigCtx, node.Done())
	if err := util.WaitError(doneCtx, errChan); err != nil {
		node.Logger.Fatal().Err(err).Msg("unhandled irrecoverable error during shutdown")
	} else if errors.Is(sigCtx.Err(), util.ErrSignalReceived) {
		node.Logger.Fatal().Msg("node shutdown aborted")
	}

	node.postShutdown()

	node.Logger.Info().Msgf("%s node shutdown complete", node.BaseConfig.NodeRole)
	os.Exit(0)
}
