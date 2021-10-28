package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
)

type Node interface {
	module.ReadyDoneAware

	// Run initiates all common components (logger, database, protocol state etc.)
	// then starts each component. It also sets up a channel to gracefully shut
	// down each component if a SIGINT is received.
	Run()

	// ShutdownSignal returns a channel that is closed when shutdown has commenced.
	ShutdownSignal() <-chan struct{}
}

type FlowNodeImp struct {
	module.ReadyDoneAware
	NodeRole string
	Logger   zerolog.Logger
	sig      chan os.Signal
}

func NewNode(builder NodeBuilder, role string, logger zerolog.Logger, sig chan os.Signal) Node {
	return &FlowNodeImp{
		ReadyDoneAware: builder,
		NodeRole:       role,
		Logger:         logger,
		sig:            sig,
	}
}

// Run calls Ready() to start all the node modules and components. It also sets up a channel to gracefully shut
// down each component if a SIGINT is received. Until a SIGINT is received, Run will block.
// Since, Run is a blocking call it should only be used when running a node as it's own independent process.
func (node *FlowNodeImp) Run() {

	// initialize signal catcher
	signal.Notify(node.sig, os.Interrupt, syscall.SIGTERM)

	select {
	case <-node.Ready():
		node.Logger.Info().Msgf("%s node startup complete", node.NodeRole)
	case <-node.sig:
		node.Logger.Warn().Msg("node startup aborted")
		os.Exit(1)
	}

	// block till a SIGINT is received
	<-node.sig

	node.Logger.Info().Msgf("%s node shutting down", node.NodeRole)

	select {
	case <-node.Done():
		node.Logger.Info().Msgf("%s node shutdown complete", node.NodeRole)
	case <-node.sig:
		node.Logger.Warn().Msg("node shutdown aborted")
		os.Exit(1)
	}

	os.Exit(0)
}

func (node *FlowNodeImp) ShutdownSignal() <-chan struct{} {
	shutdown := make(chan struct{})
	go func() {
		<-node.sig
		close(shutdown)
	}()
	return shutdown
}
