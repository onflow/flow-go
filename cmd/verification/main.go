package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

func main() {

	var (
		err       error
		receipts  mempool.Receipts
		approvals mempool.Approvals
		blocks    mempool.Blocks
	)

	cmd.FlowNode("verification").
		Create(func(node *cmd.FlowNodeBuilder) {
			receipts, err = stdmap.NewReceipts()
			node.MustNot(err).Msg("could not initialize execution receipts mempool")

			approvals, err = stdmap.NewApprovals()
			node.MustNot(err).Msg("could not initialize result approvals mempool")
		}).
		Component("verifier engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing verifier engine")

			vrfy, err := verifier.New(node.Logger, node.Network, node.State, node.Me, receipts, approvals, blocks)
			node.MustNot(err).Msg("could not initialize verifier engine")
			return vrfy
		}).
		Run()
}
