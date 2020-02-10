// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/matching"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/consensus/provider"
	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/builder/consensus"
	"github.com/dapperlabs/flow-go/module/cleaner"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		guarantees mempool.Guarantees
		receipts   mempool.Receipts
		approvals  mempool.Approvals
		seals      mempool.Seals
		prop       *propagation.Engine
		prov       *provider.Engine
		err        error
	)

	cmd.FlowNode("consensus").
		Create(func(node *cmd.FlowNodeBuilder) {
			node.Logger.Info().Msg("initializing guarantee mempool")
			guarantees, err = stdmap.NewGuarantees()
			node.MustNot(err).Msg("could not initialize guarantee mempool")
		}).
		Create(func(node *cmd.FlowNodeBuilder) {
			node.Logger.Info().Msg("initializing receipt mempool")
			receipts, err = stdmap.NewReceipts()
			node.MustNot(err).Msg("could not initialize receipt mempool")
		}).
		Create(func(node *cmd.FlowNodeBuilder) {
			node.Logger.Info().Msg("initializing approval mempool")
			approvals, err = stdmap.NewApprovals()
			node.MustNot(err).Msg("could not initialize approval mempool")
		}).
		Create(func(node *cmd.FlowNodeBuilder) {
			node.Logger.Info().Msg("initializing seal mempool")
			seals, err = stdmap.NewSeals()
			node.MustNot(err).Msg("could not initialize seal mempool")
		}).
		Component("matching engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing result matching engine")
			results := storage.NewResults(node.DB)
			match, err := matching.New(node.Logger, node.Network, node.State, node.Me, results, receipts, approvals, seals)
			node.MustNot(err).Msg("could not initialize matching engine")
			return match
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing block provider engine")
			prov, err = provider.New(node.Logger, node.Network, node.State, node.Me)
			node.MustNot(err).Msg("could not initialize provider engine")
			return prov
		}).
		Component("propagation engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing guarantee propagation engine")
			prop, err = propagation.New(node.Logger, node.Network, node.State, node.Me, guarantees)
			node.MustNot(err).Msg("could not initialize propagation engine")
			return prop
		}).
		Component("coldstuff engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing coldstuff engine")
			headersDB := storage.NewHeaders(node.DB)
			guaranteesDB := storage.NewGuarantees(node.DB)
			sealsDB := storage.NewSeals(node.DB)
			build := consensus.NewBuilder(node.DB, node.State, guarantees, seals)
			clean := cleaner.New(guaranteesDB, sealsDB, guarantees, seals)
			cold, err := coldstuff.New(node.Logger, node.Network, prop, headersDB, node.State, node.Me, build, clean)
			node.MustNot(err).Msg("could not initialize coldstuff engine")
			return cold
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			ing, err := ingestion.New(node.Logger, node.Network, prop, node.State, node.Me)
			node.MustNot(err).Msg("could not initialize guarantee ingestion engine")
			return ing
		}).
		Run()
}
