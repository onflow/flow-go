package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/components/computer"
	"github.com/dapperlabs/flow-go/engine/execution/execution/components/executor"
	context "github.com/dapperlabs/flow-go/engine/execution/execution/modules/context/mock"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/module"
	storage "github.com/dapperlabs/flow-go/storage/mock"
)

func main() {

	cmd.
		FlowNode("execution").
		Component("execution engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {

			node.Logger.Info().Msg("initializing execution engine")

			// TODO: replace mocks with real implementation
			rt := runtime.NewInterpreterRuntime()
			comp := computer.New(rt, &context.Provider{})
			exec := executor.New(comp)

			collections := &storage.Collections{}

			engine, err := execution.New(
				node.Logger,
				node.Network,
				node.Me,
				collections,
				exec,
			)
			node.MustNot(err).Msg("could not initialize execution engine")
			return engine
		}).Run()

}
