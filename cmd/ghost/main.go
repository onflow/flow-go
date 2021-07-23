package main

import (
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/engine/ghost/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/validator"
)

func main() {
	var (
		rpcConf engine.Config
	)

	cmd.FlowNode("ghost").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "r", "localhost:9000", "the address the GRPC server listens on")
		}).
		Initialize().
		PreInit(func(builder cmd.NodeBuilder, node *cmd.NodeConfig) {
			self, err := node.State.Final().Identity(node.NodeID)

			if err != nil {
				node.Logger.Warn().Msgf("node identity not found in the identity list of the finalized state: %v", node.NodeID)

				self = &flow.Identity{
					NodeID:        node.NodeID,
					NetworkPubKey: node.NetworkKey.PublicKey(),
					StakingPubKey: nil,             // no staking key needed for the unstaked node
					Role:          flow.RoleAccess, // unstaked node can only run as an access node
					Address:       node.BaseConfig.BindAddr,
				}
			}

			me, err := local.New(self, nil)
			builder.MustNot(err).Msg("could not initialize local")
			node.Me = me
		}).
		Module("message validators", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			validators := []network.MessageValidator{
				// filter out messages sent by this node itself
				validator.NewSenderValidator(node.Me.NodeID()),
				// but retain all the 1-k messages even if they are not intended for this node
			}
			node.MsgValidators = validators
			return nil
		}).
		Component("RPC engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			rpcEng, err := engine.New(node.Network, node.Logger, node.Me, node.State, rpcConf)
			return rpcEng, err
		}).
		Run()
}
