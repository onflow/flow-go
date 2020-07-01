package main

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	protocolRecovery "github.com/dapperlabs/flow-go/consensus/recovery/protocol"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/verification/finder"
	"github.com/dapperlabs/flow-go/engine/verification/match"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	"github.com/dapperlabs/flow-go/module/chunks"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/signature"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

const (
	// following lists the operational parameters of verification node
	//
	// chunkAssignmentAlpha represents number of verification
	// DISCLAIMER: alpha down there is not a production-level value
	chunkAssignmentAlpha = 1

	// requestIntervalMs represents the time interval in milliseconds that the
	// ingest engine retries sending resource requests to the network
	// this value is set following this issue:
	// https://github.com/dapperlabs/flow-go/issues/3443
	requestIntervalMs = 1000 * time.Millisecond

	// failureThreshold represents the number of retries ingest engine sends
	// at `requestIntervalMs` milliseconds for each of the missing resources.
	// When it reaches the threshold ingest engine makes a missing challenge for the resources.
	// this value is set following this issue:
	// https://github.com/dapperlabs/flow-go/issues/3443
	failureThreshold = 2
)

func main() {

	var (
		alpha               uint
		receiptLimit        uint
		chunkLimit          uint
		err                 error
		pendingReceipts     *stdmap.PendingReceipts
		pendingResults      *stdmap.PendingResults
		conCache            *buffer.PendingBlocks
		receiptIDsByBlock   *stdmap.IdentifierMap
		receiptIDsByResult  *stdmap.IdentifierMap
		pendingChunks       *match.Chunks
		headerStorage       *storage.Headers
		processedResultsIDs *stdmap.Identifiers
		finderEng           *finder.Engine
		verifierEng         *verifier.Engine
		matchEng            *match.Engine
		collector           module.VerificationMetrics
	)

	cmd.FlowNode(flow.RoleVerification.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&receiptLimit, "receipt-limit", 1000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&chunkLimit, "chunk-limit", 10000, "maximum number of chunk states in the memory pool")
			flags.UintVar(&alpha, "alpha", 10, "maximum number of chunk states in the memory pool")
		}).
		Module("verification metrics", func(node *cmd.FlowNodeBuilder) error {
			collector = metrics.NewVerificationCollector(node.Tracer, node.MetricsRegisterer, node.Logger)
			return nil
		}).
		Module("execution pending receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			pendingReceipts, err = stdmap.NewPendingReceipts(receiptLimit)
			if err != nil {
				return err
			}

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourcePendingReceipt, pendingReceipts.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("pending receipt ids by block mempool", func(node *cmd.FlowNodeBuilder) error {
			receiptIDsByBlock, err = stdmap.NewIdentifierMap(receiptLimit)
			if err != nil {
				return err
			}

			return nil
		}).
		Module("pending receipt ids by result mempool", func(node *cmd.FlowNodeBuilder) error {
			receiptIDsByResult, err = stdmap.NewIdentifierMap(receiptLimit)
			if err != nil {
				return err
			}

			return nil
		}).
		Module("pending results mempool", func(node *cmd.FlowNodeBuilder) error {
			pendingResults = stdmap.NewPendingResults()

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourcePendingResult, pendingResults.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("match chunks mempool", func(node *cmd.FlowNodeBuilder) error {
			pendingChunks = match.NewChunks(chunkLimit)

			err = node.Metrics.Mempool.Register(metrics.ResourcePendingChunks, pendingChunks.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("processed results ids mempool", func(node *cmd.FlowNodeBuilder) error {
			processedResultsIDs, err = stdmap.NewIdentifiers(receiptLimit)
			if err != nil {
				return err
			}
			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceProcessedResultIDs, processedResultsIDs.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("block cache", func(node *cmd.FlowNodeBuilder) error {
			// consensus cache for follower engine
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		Module("header storage", func(node *cmd.FlowNodeBuilder) error {
			headerStorage = storage.NewHeaders(node.Metrics.Cache, node.DB)
			return nil
		}).
		Component("verifier engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rt := runtime.NewInterpreterRuntime()
			vm, err := virtualmachine.New(rt, node.RootChainID.Chain())
			if err != nil {
				return nil, err
			}
			chunkVerifier := chunks.NewChunkVerifier(vm, node.Storage.Blocks)
			verifierEng, err = verifier.New(node.Logger, collector, node.Network, node.State, node.Me, chunkVerifier)
			return verifierEng, err
		}).
		Component("match engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			assigner, err := chunks.NewPublicAssignment(chunkAssignmentAlpha)
			if err != nil {
				return nil, err
			}
			matchEng, err = match.New(node.Logger,
				collector,
				node.Network,
				node.Me,
				pendingResults,
				verifierEng,
				assigner,
				node.State,
				pendingChunks,
				headerStorage,
				requestIntervalMs,
				failureThreshold)
			return matchEng, err
		}).
		Component("finder engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			finderEng, err = finder.New(node.Logger,
				collector,
				node.Network,
				node.Me,
				matchEng,
				pendingReceipts,
				headerStorage,
				processedResultsIDs,
				receiptIDsByBlock,
				receiptIDsByResult)
			return finderEng, err
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			// TODO frequency of 0 turns off the cleaner, turn back on once we know the proper tuning
			cleaner := storage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)

			// create a finalizer that handles updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, node.Storage.Payloads, node.State)

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

			finalized, pending, err := protocolRecovery.FindLatest(node.State, node.Storage.Headers, node.GenesisBlock.Header)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			core, err := consensus.NewFollower(node.Logger, mainConsensusCommittee, node.Storage.Headers, final, verifier, finderEng, node.GenesisBlock.Header, node.GenesisQC, finalized, pending)
			if err != nil {
				// return nil, fmt.Errorf("could not create follower core logic: %w", err)
				// TODO for now we ignore failures in follower
				// this is necessary for integration tests to run, until they are
				// updated to generate/use valid genesis QC and DKG files.
				// ref https://github.com/dapperlabs/flow-go/issues/3057
				node.Logger.Debug().Err(err).Msg("ignoring failures in follower core")
			}

			followerEng, err := followereng.New(node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				node.State,
				conCache,
				core)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			// create a block synchronization engine to handle follower getting
			// out of sync
			sync, err := synchronization.New(node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				followerEng)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			return followerEng.WithSynchronization(sync), nil
		}).
		Run()
}
