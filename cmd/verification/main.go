package main

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/verification/finder"
	"github.com/onflow/flow-go/engine/verification/match"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/chunks"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	storage "github.com/onflow/flow-go/storage/badger"
)

const (
	// requestInterval represents the time interval in milliseconds that the
	// match engine retries sending resource requests to the network
	// this value is set following this issue (3443)
	requestInterval = 1000 * time.Millisecond

	// processInterval represents the time interval in milliseconds that the
	// finder engine iterates over the execution receipts ready to process
	// this value is set following this issue (3443)
	processInterval = 1000 * time.Millisecond

	// failureThreshold represents the number of retries match engine sends
	// at `requestInterval` milliseconds for each of the missing resources.
	// When it reaches the threshold ingest engine makes a missing challenge for the resources.
	// Currently setting the threshold to a very large value (corresponding to 100 days),
	// which for all practical purposes is equivalent to the Verifier trying indefinitely.
	failureThreshold = 10000000
)

func main() {
	var (
		followerState       protocol.MutableState
		err                 error
		receiptLimit        uint                       // size of execution-receipt/result related mempools
		chunkAlpha          uint                       // number of verifiers assigned per chunk
		chunkLimit          uint                       // size of chunk-related mempools
		cachedReceipts      *stdmap.ReceiptDataPacks   // used in finder engine
		pendingReceipts     *stdmap.ReceiptDataPacks   // used in finder engine
		readyReceipts       *stdmap.ReceiptDataPacks   // used in finder engine
		blockIDsCache       *stdmap.Identifiers        // used in finder engine
		processedResultsIDs *stdmap.Identifiers        // used in finder engine
		discardedResultIDs  *stdmap.Identifiers        // used in finder engine
		receiptIDsByBlock   *stdmap.IdentifierMap      // used in finder engine
		receiptIDsByResult  *stdmap.IdentifierMap      // used in finder engine
		chunkIDsByResult    *stdmap.IdentifierMap      // used in match engine
		pendingResults      *stdmap.ResultDataPacks    // used in match engine
		pendingChunks       *match.Chunks              // used in match engine
		headerStorage       *storage.Headers           // used in match and finder engines
		syncCore            *synchronization.Core      // used in follower engine
		pendingBlocks       *buffer.PendingBlocks      // used in follower engine
		finderEng           *finder.Engine             // the finder engine
		verifierEng         *verifier.Engine           // the verifier engine
		matchEng            *match.Engine              // the match engine
		followerEng         *followereng.Engine        // the follower engine
		collector           module.VerificationMetrics // used to collect metrics of all engines
	)

	cmd.FlowNode(flow.RoleVerification.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&receiptLimit, "receipt-limit", 1000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&chunkLimit, "chunk-limit", 10000, "maximum number of chunk states in the memory pool")
			flags.UintVar(&chunkAlpha, "chunk-alpha", chunks.DefaultChunkAssignmentAlpha, "number of verifiers that should be assigned to each chunk")
		}).
		Module("mutable follower state", func(node *cmd.FlowNodeBuilder) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currenlty supported but read-only state has type %T", node.State)
			}
			followerState, err = badgerState.NewFollowerState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
			)
			return err
		}).
		Module("verification metrics", func(node *cmd.FlowNodeBuilder) error {
			collector = metrics.NewVerificationCollector(node.Tracer, node.MetricsRegisterer)
			return nil
		}).
		Module("cached execution receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			cachedReceipts, err = stdmap.NewReceiptDataPacks(receiptLimit)
			if err != nil {
				return err
			}

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceCachedReceipt, cachedReceipts.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("pending execution receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			pendingReceipts, err = stdmap.NewReceiptDataPacks(receiptLimit)
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
		Module("ready execution receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			readyReceipts, err = stdmap.NewReceiptDataPacks(receiptLimit)
			if err != nil {
				return err
			}

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceReceipt, readyReceipts.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("pending execution receipts ids by block mempool", func(node *cmd.FlowNodeBuilder) error {
			receiptIDsByBlock, err = stdmap.NewIdentifierMap(receiptLimit)
			if err != nil {
				return err
			}

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourcePendingReceiptIDsByBlock, receiptIDsByBlock.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}

			return nil
		}).
		Module("execution receipt ids by result mempool", func(node *cmd.FlowNodeBuilder) error {
			receiptIDsByResult, err = stdmap.NewIdentifierMap(receiptLimit)
			if err != nil {
				return err
			}

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceReceiptIDsByResult, receiptIDsByResult.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}

			return nil
		}).
		Module("chunk ids by result mempool", func(node *cmd.FlowNodeBuilder) error {
			chunkIDsByResult, err = stdmap.NewIdentifierMap(chunkLimit)
			if err != nil {
				return err
			}

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceChunkIDsByResult, chunkIDsByResult.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}

			return nil
		}).
		Module("cached block ids mempool", func(node *cmd.FlowNodeBuilder) error {
			blockIDsCache, err = stdmap.NewIdentifiers(receiptLimit)
			if err != nil {
				return err
			}

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceCachedBlockID, blockIDsCache.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}

			return nil
		}).
		Module("pending results mempool", func(node *cmd.FlowNodeBuilder) error {
			pendingResults = stdmap.NewResultDataPacks(receiptLimit)

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourcePendingResult, pendingResults.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("pending chunks mempool", func(node *cmd.FlowNodeBuilder) error {
			pendingChunks = match.NewChunks(chunkLimit)

			err = node.Metrics.Mempool.Register(metrics.ResourcePendingChunk, pendingChunks.Size)
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
			err = node.Metrics.Mempool.Register(metrics.ResourceProcessedResultID, processedResultsIDs.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("discarded results ids mempool", func(node *cmd.FlowNodeBuilder) error {
			discardedResultIDs, err = stdmap.NewIdentifiers(receiptLimit)
			if err != nil {
				return err
			}
			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceDiscardedResultID, discardedResultIDs.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("pending block cache", func(node *cmd.FlowNodeBuilder) error {
			// consensus cache for follower engine
			pendingBlocks = buffer.NewPendingBlocks()

			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourcePendingBlock, pendingBlocks.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}

			return nil
		}).
		Module("header storage", func(node *cmd.FlowNodeBuilder) error {
			headerStorage = storage.NewHeaders(node.Metrics.Cache, node.DB)
			return nil
		}).
		Module("sync core", func(node *cmd.FlowNodeBuilder) error {
			syncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Component("verifier engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rt := fvm.NewInterpreterRuntime()
			vm := fvm.NewVirtualMachine(rt)
			vmCtx := fvm.NewContext(node.Logger, node.FvmOptions...)
			chunkVerifier := chunks.NewChunkVerifier(vm, vmCtx)
			approvalStorage := storage.NewResultApprovals(node.Metrics.Cache, node.DB)
			verifierEng, err = verifier.New(
				node.Logger,
				collector,
				node.Tracer,
				node.Network,
				node.State,
				node.Me,
				chunkVerifier,
				approvalStorage)
			return verifierEng, err
		}).
		Component("match engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			assigner, err := chunks.NewChunkAssigner(chunkAlpha, node.State)
			if err != nil {
				return nil, err
			}
			matchEng, err = match.New(node.Logger,
				collector,
				node.Tracer,
				node.Network,
				node.Me,
				pendingResults,
				chunkIDsByResult,
				verifierEng,
				assigner,
				node.State,
				pendingChunks,
				headerStorage,
				requestInterval,
				failureThreshold)
			return matchEng, err
		}).
		Component("finder engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			finderEng, err = finder.New(node.Logger,
				collector,
				node.Tracer,
				node.Network,
				node.Me,
				node.State,
				matchEng,
				cachedReceipts,
				pendingReceipts,
				readyReceipts,
				headerStorage,
				processedResultsIDs,
				discardedResultIDs,
				receiptIDsByBlock,
				receiptIDsByResult,
				blockIDsCache,
				processInterval)
			return finderEng, err
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			cleaner := storage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)

			// create a finalizer that handles updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, followerState)

			// initialize the staking & beacon verifiers, signature joiner
			staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
			beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
			merger := signature.NewCombiner()

			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			committee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(committee, staking, beacon, merger)

			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			followerCore, err := consensus.NewFollower(node.Logger, committee, node.Storage.Headers, final, verifier, finderEng, node.RootBlock.Header, node.RootQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not create follower core logic: %w", err)
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
				followerState,
				pendingBlocks,
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
