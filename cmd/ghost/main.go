package main

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/ghost/engine"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	jsoncodec "github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/validators"
)

func main() {
	var (
		rpcConf engine.Config
	)

	cmd.FlowNode("ghost").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "r", "localhost:9000", "the address the GRPC server listens on")
		}).
		// overwriting the network created by NodeBuilder and using custom list of validators and topology
		Component("network", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			codec := jsoncodec.NewCodec()

			validator := []validators.MessageValidator{
				// validator to filter out messages sent by this node itself
				validators.NewSenderValidator(node.NodeID),
				// not adding TargetValidator since we want to receive all messages including the ones not targeted
				// for this node
			}

			mw, err := libp2p.NewMiddleware(node.Logger.Level(zerolog.ErrorLevel), codec, node.Me.Address(), node.Me.NodeID(), node.NetworkKey, validator...)
			if err != nil {
				return nil, fmt.Errorf("could not initialize middleware: %w", err)
			}

			ids, err := node.State.Final().Identities(filter.Any)
			if err != nil {
				return nil, fmt.Errorf("could not get network identities: %w", err)
			}

			net, err := libp2p.NewNetwork(node.Logger, codec, ids, node.Me, mw, 10e6, engine.NewAllConnectTopology())
			if err != nil {
				return nil, fmt.Errorf("could not initialize network: %w", err)
			}

			node.Network = net
			return net, err
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng, err := engine.New(node.Network, node.Logger, node.Me, rpcConf)
			return rpcEng, err
		}).
		Run()
}
