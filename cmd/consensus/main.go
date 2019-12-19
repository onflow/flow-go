// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff"
	"github.com/dapperlabs/flow-go/engine/simulation/generator"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
)

func main() {

	var (
		pool module.CollectionPool
		prop *propagation.Engine
		err  error
	)

	cmd.FlowNode("consensus").
		Create(func(node *cmd.FlowNodeBuilder) {
			pool, err = mempool.NewCollectionPool()
			node.MustNot(err).Msg("could not initialize engine mempool")
		}).
		Component("propagation engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing propagation engine")

			prop, err = propagation.New(node.Logger, node.Network, node.State, node.Me, pool)
			node.MustNot(err).Msg("could not initialize propagation engine")
			return prop
		}).
		Component("coldstuff engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			cold, err := coldstuff.New(node.Logger, node.Network, node.State, node.Me, pool)
			node.MustNot(err).Msg("could not initialize coldstuff engine")
			return cold
		}).
		Component("generator engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			gen, err := generator.New(node.Logger, prop)
			node.MustNot(err).Msg("could not initialize generator engine")
			return gen
		}).
		Run()
}
