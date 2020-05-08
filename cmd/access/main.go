package main

import (
	"fmt"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-go/module/metrics"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/engine/access/ingestion"
	"github.com/dapperlabs/flow-go/engine/access/rpc"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/signature"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		blockLimit      uint
		collectionLimit uint
		receiptLimit    uint
		ingestEng       *ingestion.Engine
		rpcConf         rpc.Config
		collectionRPC   access.AccessAPIClient
		executionRPC    execution.ExecutionAPIClient
		err             error
		collections     *storage.Collections
		transactions    *storage.Transactions
		conCache        *buffer.PendingBlocks // pending block cache for follower
	)

	cmd.FlowNode("access").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&receiptLimit, "receipt-limit", 1000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&collectionLimit, "collection-limit", 1000, "maximum number of collections in the memory pool")
			flags.UintVar(&blockLimit, "block-limit", 1000, "maximum number of result blocks in the memory pool")
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "r", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVarP(&rpcConf.CollectionAddr, "ingress-addr", "i", "localhost:9000", "the address (of the collection node) to send transactions to")
			flags.StringVarP(&rpcConf.ExecutionAddr, "script-addr", "s", "localhost:9000", "the address (of the execution node) forward the script to")
		}).
		Module("collection node client", func(node *cmd.FlowNodeBuilder) error {
			node.Logger.Info().Err(err).Msgf("Collection node Addr: %s", rpcConf.CollectionAddr)

			collectionRPCConn, err := grpc.Dial(rpcConf.CollectionAddr, grpc.WithInsecure())
			if err != nil {
				return err
			}
			collectionRPC = access.NewAccessAPIClient(collectionRPCConn)
			return nil
		}).
		Module("execution node client", func(node *cmd.FlowNodeBuilder) error {
			node.Logger.Info().Err(err).Msgf("Execution node Addr: %s", rpcConf.ExecutionAddr)

			executionRPCConn, err := grpc.Dial(rpcConf.ExecutionAddr, grpc.WithInsecure())
			if err != nil {
				return err
			}
			executionRPC = execution.NewExecutionAPIClient(executionRPCConn)
			return nil
		}).
		Module("persistent storage", func(node *cmd.FlowNodeBuilder) error {
			collections = storage.NewCollections(node.DB)
			transactions = storage.NewTransactions(node.DB)
			return nil
		}).
		Module("block cache", func(node *cmd.FlowNodeBuilder) error {
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		Module("metrics collector", func(node *cmd.FlowNodeBuilder) error {
			node.Metrics, err = metrics.NewCollector(node.Logger)
			return err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ingestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Metrics, node.Me, node.Blocks, node.Headers, collections, transactions)
			return ingestEng, err
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			cleaner := storage.NewCleaner(node.Logger, node.DB)

			// create a finalizer that will handle updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Headers, node.Payloads, node.State)

			// initialize the staking & beacon verifiers, signature joiner
			staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
			beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
			merger := signature.NewCombiner()

			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			mainConsensusCommittee, err := committee.NewMainConsensusCommitteeState(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(mainConsensusCommittee, node.DKGState, staking, beacon, merger)

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			core, err := consensus.NewFollower(node.Logger, mainConsensusCommittee, final, verifier, ingestEng, node.GenesisBlock.Header, node.GenesisQC)
			if err != nil {
				// TODO for now we ignore failures in follower
				// this is necessary for integration tests to run, until they are
				// updated to generate/use valid genesis QC and DKG files.
				// ref https://github.com/dapperlabs/flow-go/issues/3057
				node.Logger.Debug().Err(err).Msg("ignoring failures in follower core")
			}

			follower, err := followereng.New(node.Logger, node.Network, node.Me, cleaner, node.Headers, node.Payloads, node.State, conCache, core)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			// create a block synchronization engine to handle follower getting
			// out of sync
			sync, err := synchronization.New(node.Logger, node.Network, node.Me, node.State, node.Blocks, follower)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			return follower.WithSynchronization(sync), nil
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, node.State, rpcConf, executionRPC, collectionRPC, node.Blocks, node.Headers, collections, transactions)
			return rpcEng, nil
		}).
		Run(flow.RoleAccess.String())
}
