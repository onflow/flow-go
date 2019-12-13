// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff"
	"github.com/dapperlabs/flow-go/engine/simulation/generator"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"

	. "github.com/dapperlabs/flow-go/cmd"
)

func main() {

	var state *protocol.State
	var pool *mempool.Mempool
	var prop *propagation.Engine
	var err error

	FlowNode("consensus").
		Create(func(node *FlowNodeBuilder) {
			pool, err = mempool.New()
			node.MustNot(err).Msg("could not initialize engine mempool")
		}).
		CreateReadDoneAware("propagation engine", func(node *FlowNodeBuilder) network.ReadyDoneAware {
			node.Logger.Info().Msg("initializing propagation engine")

			prop, err = propagation.New(node.Logger, node.Network, state, node.Me, pool)
			node.MustNot(err).Msg("could not initialize propagation engine")
			return prop
		}).
		CreateReadDoneAware("coldstuff engine", func(node *FlowNodeBuilder) network.ReadyDoneAware {
			cold, err := coldstuff.New(node.Logger, node.Network, state, node.Me, pool)
			node.MustNot(err).Msg("could not initialize coldstuff engine")
			return cold
		}).
		CreateReadDoneAware("generator engine", func(node *FlowNodeBuilder) network.ReadyDoneAware {
			gen, err := generator.New(node.Logger, prop)
			node.MustNot(err).Msg("could not initialize generator engine")
			return gen
		}).
		Run()
}
