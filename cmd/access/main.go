package main

import (
	"fmt"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	recovery "github.com/dapperlabs/flow-go/consensus/recovery/protocol"
	"github.com/dapperlabs/flow-go/engine/access/ingestion"
	"github.com/dapperlabs/flow-go/engine/access/rpc"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	synceng "github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/module/synchronization"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	grpcutils "github.com/dapperlabs/flow-go/utils/grpc"
)

func main() {

	var (
		blockLimit      uint
		collectionLimit uint
		receiptLimit    uint
		ingestEng       *ingestion.Engine
		followerEng     *followereng.Engine
		syncCore        *synchronization.Core
		rpcConf         rpc.Config
		rpcEng          *rpc.Engine
		collectionRPC   access.AccessAPIClient
		executionRPC    execution.ExecutionAPIClient
		err             error
		conCache        *buffer.PendingBlocks // pending block cache for follower
	)

	cmd.FlowNode(flow.RoleAccess.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&receiptLimit, "receipt-limit", 1000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&collectionLimit, "collection-limit", 1000, "maximum number of collections in the memory pool")
			flags.UintVar(&blockLimit, "block-limit", 1000, "maximum number of result blocks in the memory pool")
			flags.StringVarP(&rpcConf.GRPCListenAddr, "rpc-addr", "r", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVarP(&rpcConf.HTTPListenAddr, "http-addr", "h", "localhost:8000", "the address the http proxy server listens on")
			flags.StringVarP(&rpcConf.CollectionAddr, "ingress-addr", "i", "localhost:9000", "the address (of the collection node) to send transactions to")
			flags.StringVarP(&rpcConf.ExecutionAddr, "script-addr", "s", "localhost:9000", "the address (of the execution node) forward the script to")
		}).
		Module("collection node client", func(node *cmd.FlowNodeBuilder) error {
			node.Logger.Info().Err(err).Msgf("Collection node Addr: %s", rpcConf.CollectionAddr)

			collectionRPCConn, err := grpc.Dial(
				rpcConf.CollectionAddr,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure())
			if err != nil {
				return err
			}
			collectionRPC = access.NewAccessAPIClient(collectionRPCConn)
			return nil
		}).
		Module("execution node client", func(node *cmd.FlowNodeBuilder) error {
			node.Logger.Info().Err(err).Msgf("Execution node Addr: %s", rpcConf.ExecutionAddr)

			executionRPCConn, err := grpc.Dial(
				rpcConf.ExecutionAddr,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure())
			if err != nil {
				return err
			}
			executionRPC = execution.NewExecutionAPIClient(executionRPCConn)
			return nil
		}).
		Module("block cache", func(node *cmd.FlowNodeBuilder) error {
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		Module("sync core", func(node *cmd.FlowNodeBuilder) error {
			syncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng = rpc.New(node.Logger, node.State, rpcConf, executionRPC, collectionRPC, node.Storage.Blocks, node.Storage.Headers, node.Storage.Collections, node.Storage.Transactions, node.RootChainID)
			return rpcEng, nil
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ingestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Me, node.Storage.Blocks, node.Storage.Headers, node.Storage.Collections, node.Storage.Transactions, rpcEng)
			return ingestEng, err
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			// TODO frequency of 0 turns off the cleaner, turn back on once we know the proper tuning
			cleaner := storage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)

			// create a finalizer that will handle updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, node.State)

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

			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			followerCore, err := consensus.NewFollower(node.Logger, mainConsensusCommittee, node.Storage.Headers, final, verifier, ingestEng, node.RootBlock.Header, node.RootQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not initialize follower core: %w", err)
			}

			followerEng, err = followereng.New(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				node.State,
				conCache,
				followerCore,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			return followerEng, nil
		}).
		Component("sync engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			sync, err := synceng.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				followerEng,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}
			return sync, nil
		}).
		Run()
}
