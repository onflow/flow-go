// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/persister"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/consensus/compliance"
	"github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/matching"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/consensus/provider"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	builder "github.com/dapperlabs/flow-go/module/builder/consensus"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		guaranteeLimit  uint
		resultLimit     uint
		receiptLimit    uint
		approvalLimit   uint
		sealLimit       uint
		minInterval     time.Duration
		maxInterval     time.Duration
		hotstuffTimeout time.Duration

		err            error
		privateDKGData *bootstrap.DKGParticipantPriv
		guarantees     mempool.Guarantees
		results        mempool.Results
		receipts       mempool.Receipts
		approvals      mempool.Approvals
		seals          mempool.Seals
		prop           *propagation.Engine
		prov           *provider.Engine
		sync           *synchronization.Engine
		conMetrics     module.ConsensusMetrics
		mainMetrics    module.HotstuffMetrics
	)

	cmd.FlowNode("consensus").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&guaranteeLimit, "guarantee-limit", 10000, "maximum number of guarantees in the memory pool")
			flags.UintVar(&resultLimit, "result-limit", 1000, "maximum number of execution results in the memory pool")
			flags.UintVar(&receiptLimit, "receipt-limit", 1000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&approvalLimit, "approval-limit", 1000, "maximum number of result approvals in the memory pool")
			flags.UintVar(&sealLimit, "seal-limit", 1000, "maximum number of block seals in the memory pool")
			flags.DurationVar(&minInterval, "min-interval", time.Millisecond, "the minimum amount of time between two blocks")
			flags.DurationVar(&maxInterval, "max-interval", 60*time.Second, "the maximum amount of time between two blocks")
			flags.DurationVar(&hotstuffTimeout, "hotstuff-timeout", 2*time.Second, "the initial timeout for the hotstuff pacemaker")
		}).
		Module("random beacon key", func(node *cmd.FlowNodeBuilder) error {
			privateDKGData, err = loadDKGPrivateData(node.BaseConfig.BootstrapDir, node.NodeID)
			return err
		}).
		Module("collection guarantees mempool", func(node *cmd.FlowNodeBuilder) error {
			guarantees, err = stdmap.NewGuarantees(guaranteeLimit)
			return err
		}).
		Module("execution results mempool", func(node *cmd.FlowNodeBuilder) error {
			results, err = stdmap.NewResults(resultLimit)
			return err
		}).
		Module("execution receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			receipts, err = stdmap.NewReceipts(receiptLimit)
			return err
		}).
		Module("result approvals mempool", func(node *cmd.FlowNodeBuilder) error {
			approvals, err = stdmap.NewApprovals(approvalLimit)
			return err
		}).
		Module("block seals mempool", func(node *cmd.FlowNodeBuilder) error {
			seals, err = stdmap.NewSeals(sealLimit)
			return err
		}).
		Module("consensus node metrics", func(node *cmd.FlowNodeBuilder) error {
			conMetrics = metrics.NewConsensusCollector(node.Tracer)
			return nil
		}).
		Module("hotstuff main metrics", func(node *cmd.FlowNodeBuilder) error {
			mainMetrics = metrics.NewHotstuffCollector(flow.DefaultChainID)
			return nil
		}).
		Component("matching engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			resultsDB := bstorage.NewExecutionResults(node.DB)
			match, err := matching.New(
				node.Logger,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				node.Network,
				node.State,
				node.Me,
				resultsDB,
				node.Storage.Headers,
				results,
				receipts,
				approvals,
				seals,
			)
			return match, err
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			prov, err = provider.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.State,
				node.Me,
			)
			return prov, err
		}).
		Component("propagation engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			prop, err = propagation.New(
				node.Logger,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				conMetrics,
				node.Network,
				node.State,
				node.Me,
				guarantees,
			)
			return prop, err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ing, err := ingestion.New(
				node.Logger,
				node.Metrics.Engine,
				conMetrics,
				node.Network,
				prop,
				node.State,
				node.Storage.Headers,
				node.Me,
			)
			return ing, err
		}).
		Component("consensus components", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// TODO: we should probably find a way to initialize mutually dependent engines separately

			// initialize the entity database accessors
			cleaner := bstorage.NewCleaner(node.Logger, node.DB)

			// initialize the pending blocks cache
			proposals := buffer.NewPendingBlocks()

			// initialize the compliance engine
			comp, err := compliance.New(
				node.Logger,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				conMetrics,
				node.Network,
				node.Me,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				node.State,
				prov,
				proposals,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize compliance engine: %w", err)
			}

			// initialize the synchronization engine
			sync, err = synchronization.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				comp,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
			}

			// initialize the block builder
			build := builder.NewBuilder(
				node.Metrics.Mempool,
				node.DB,
				node.Storage.Headers,
				node.Storage.Seals,
				node.Storage.Index,
				node.Storage.Blocks,
				guarantees,
				seals,
				builder.WithMinInterval(minInterval),
				builder.WithMaxInterval(maxInterval),
			)

			// initialize the block finalizer
			finalize := finalizer.NewFinalizer(
				node.DB,
				node.Storage.Headers,
				node.Storage.Payloads,
				node.State,
				finalizer.WithCleanup(finalizer.CleanupMempools(
					node.Metrics.Mempool,
					conMetrics,
					node.Storage.Payloads,
					guarantees,
					seals,
				)),
			)

			// initialize the aggregating signature module for staking signatures
			staking := signature.NewAggregationProvider(encoding.ConsensusVoteTag, node.Me)

			// initialize the threshold signature module for random beacon signatures
			beacon := signature.NewThresholdProvider(encoding.RandomBeaconTag, privateDKGData.RandomBeaconPrivKey)

			// initialize the simple merger to combine staking & beacon signatures
			merger := signature.NewCombiner()

			// initialize Main consensus committee's state
			committee, err := committee.NewMainConsensusCommitteeState(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			// initialize the combined signer for hotstuff
			signer := verification.NewCombinedSigner(
				committee,
				node.DKGState,
				staking,
				beacon,
				merger,
				node.NodeID,
			)

			// initialize a logging notifier for hotstuff
			notifier := createNotifier(node.Logger, mainMetrics)
			// initialize the persister
			persist := persister.New(node.DB)

			// query the last finalized block and pending blocks for recovery
			finalized, pending, err := findLatest(node.State, node.Storage.Headers, node.GenesisBlock.Header)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks: %w", err)
			}

			// initialize hotstuff consensus algorithm
			hot, err := consensus.NewParticipant(
				node.Logger,
				notifier,
				mainMetrics,
				node.Storage.Headers,
				committee,
				build,
				finalize,
				persist,
				signer,
				comp,
				node.GenesisBlock.Header,
				node.GenesisQC,
				finalized,
				pending,
				consensus.WithTimeout(hotstuffTimeout),
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize hotstuff engine: %w", err)
			}

			comp = comp.WithSynchronization(sync).WithConsensus(hot)
			return comp, nil
		}).
		Run(flow.RoleConsensus.String())
}

func loadDKGPrivateData(dir string, myID flow.Identifier) (*bootstrap.DKGParticipantPriv, error) {
	path := fmt.Sprintf(bootstrap.PathRandomBeaconPriv, myID)
	data, err := ioutil.ReadFile(filepath.Join(dir, path))
	if err != nil {
		return nil, err
	}

	var priv bootstrap.DKGParticipantPriv
	err = json.Unmarshal(data, &priv)
	if err != nil {
		return nil, err
	}
	return &priv, nil
}

func findLatest(state protocol.State, headers storage.Headers, rootHeader *flow.Header) (*flow.Header, []*flow.Header, error) {
	// find finalized block
	finalized, err := state.Final().Head()

	if err != nil {
		return nil, nil, fmt.Errorf("could not find finalized block")
	}

	// find all pending blockIDs
	pendingIDs, err := state.Final().Pending()
	if err != nil {
		return nil, nil, fmt.Errorf("could not find pending block")
	}

	// find all pending header by ID
	pending := make([]*flow.Header, 0, len(pendingIDs))
	for _, pendingID := range pendingIDs {
		pendingHeader, err := headers.ByBlockID(pendingID)
		if err != nil {
			return nil, nil, fmt.Errorf("could not find pending block by ID: %w", err)
		}
		pending = append(pending, pendingHeader)
	}

	return finalized, pending, nil
}
