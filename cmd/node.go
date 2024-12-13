package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/utils/logging"
)

var _ component.Component = (*FlowNodeImp)(nil)

type Node interface {
	component.Component

	// Run initiates all common components (logger, database, protocol state etc.)
	// then starts each component. It also sets up a channel to gracefully shut
	// down each component if a SIGINT is received.
	// The context can also be used to signal the node to shutdown.
	Run(ctx context.Context)
}

// FlowNodeImp is created by the FlowNodeBuilder with all components ready to be started.
// The Run function starts all the components, and is blocked until either a termination
// signal is received or a irrecoverable error is encountered.
type FlowNodeImp struct {
	NodeImp
	*NodeConfig
}

// NodeImp can be used to create a node instance from:
//   - a logger: to be used during startup and shutdown
//   - a component: that will be started with Run
//   - a cleanup function: that will be called after the component has been stopped
//   - a fatal error handler: to handle any error received from the component
type NodeImp struct {
	component.Component
	logger       zerolog.Logger
	postShutdown func() error
	fatalHandler func(error)
}

// NewNode returns a new node instance
func NewNode(
	component component.Component,
	cfg *NodeConfig,
	logger zerolog.Logger,
	cleanup func() error,
	handleFatal func(error),
) Node {
	return &FlowNodeImp{
		NodeConfig: cfg,
		NodeImp: NewBaseNode(
			component,
			logger.With().
				Str("node_role", cfg.BaseConfig.NodeRole).
				Hex("spork_id", logging.ID(cfg.SporkID)).
				Logger(),
			cleanup,
			handleFatal,
		),
	}
}

// NewBaseNode returns a new base node instance
func NewBaseNode(
	component component.Component,
	logger zerolog.Logger,
	cleanup func() error,
	handleFatal func(error),
) NodeImp {
	return NodeImp{
		Component:    component,
		logger:       logger,
		postShutdown: cleanup,
		fatalHandler: handleFatal,
	}
}

// Run starts all the node's components, then blocks until a SIGINT or SIGTERM is received, at
// which point it gracefully shuts down.
// Any unhandled irrecoverable errors thrown in child components will propagate up to here and
// result in a fatal error.
func (node *NodeImp) Run(ctx context.Context) {

	// Block until node is shutting down
	err := node.run(ctx)

	// Any error received is considered fatal.
	if err != nil {
		node.fatalHandler(err)
		return
	}

	// Run post shutdown cleanup logic
	err = node.postShutdown()

	// Since this occurs after all components have stopped, it is not considered fatal
	if err != nil {
		node.logger.Error().Err(err).Msg("error encountered during cleanup")
	}

	node.logger.Info().Msg("node shutdown complete")
}

// run starts the node and blocks until a SIGINT/SIGTERM is received or an error is encountered.
// It returns:
//   - nil if a termination signal is received, and all components have been gracefully stopped.
//   - error if an irrecoverable error is received
func (node *NodeImp) run(ctx context.Context) error {
	// Cancelling this context notifies all child components that it's time to shut down
	ctx, shutdown := context.WithCancel(ctx)
	defer shutdown()

	// Components will pass unhandled irrecoverable errors to this channel via signalerCtx (or a
	// child context). Any errors received on this channel should halt the node.
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	// This context will be marked done when SIGINT/SIGTERM is received.
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	// 1: Start up
	// Start all the components
	node.Start(signalerCtx)

	// Log when all components have been started
	go func() {
		select {
		case <-node.Ready():
			node.logger.Info().
				Msg("node startup complete")
		case <-ctx.Done():
		}
	}()

	// 2: Run the node
	// Block here until either a signal or irrecoverable error is received.
	err := util.WaitError(errChan, sigCtx.Done())

	// Stop relaying signals. Subsequent signals will be handled by the OS and will abort the
	// process.
	stop()

	// If an irrecoverable error was received, abort
	if err != nil {
		return err
	}

	// 3: Shut down
	// Send shutdown signal to components
	node.logger.Info().Msg("node shutting down")
	shutdown()

	// Block here until all components have stopped or an irrecoverable error is received.
	return util.WaitError(errChan, node.Done())
}
