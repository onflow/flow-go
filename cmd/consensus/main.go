// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/consensus/consensus"
	"github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/matching"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/consensus/provider"
	"github.com/dapperlabs/flow-go/module"
	builder "github.com/dapperlabs/flow-go/module/builder/consensus"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		err        error
		chainID    string
		guarantees mempool.Guarantees
		receipts   mempool.Receipts
		approvals  mempool.Approvals
		seals      mempool.Seals
		prop       *propagation.Engine
		prov       *provider.Engine
	)

	cmd.FlowNode("consensus").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&chainID, "chain-id", "C", "flow", "the chain ID for the protocol chain")
		}).
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
			payloadsDB := storage.NewPayloads(node.DB)
			build := builder.NewBuilder(node.DB, guarantees, seals, chainID)
			final := finalizer.NewFinalizer(node.DB, guarantees, seals, prov)

			cons, err := consensus.New(node.Logger, node.Network, node.Me, node.State, headersDB, payloadsDB, build, final)
			node.MustNot(err).Msg("could not initialize consensus engine")
			return cons
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			ing, err := ingestion.New(node.Logger, node.Network, prop, node.State, node.Me)
			node.MustNot(err).Msg("could not initialize guarantee ingestion engine")
			return ing
		}).
		Run()
}
