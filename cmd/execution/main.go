package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/components/computer"
	"github.com/dapperlabs/flow-go/engine/execution/execution/components/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/modules/context"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/module"
	storage "github.com/dapperlabs/flow-go/storage/mock"
)

func main() {

	cmd.
		FlowNode("execution").
		Component("execution engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {

			node.Logger.Info().Msg("initializing execution engine")

			rt := runtime.NewInterpreterRuntime()
			comp := computer.New(rt, context.NewProvider())
			exec := executor.New(comp)

			// TODO: replace mocks with real implementation
			transactions := &storage.Transactions{}
			collections := &storage.Collections{}

			engine, err := execution.New(
				node.Logger,
				node.Network,
				node.Me,
				collections,
				transactions,
				exec,
			)
			node.MustNot(err).Msg("could not initialize execution engine")
			return engine
		}).Run()

}
