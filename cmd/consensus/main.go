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
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	builder "github.com/dapperlabs/flow-go/module/builder/consensus"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/signature"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		guaranteeLimit  uint
		receiptLimit    uint
		approvalLimit   uint
		sealLimit       uint
		chainID         string
		minInterval     time.Duration
		maxInterval     time.Duration
		hotstuffTimeout time.Duration

		err            error
		privateDKGData *bootstrap.DKGParticipantPriv
		guarantees     mempool.Guarantees
		receipts       mempool.Receipts
		approvals      mempool.Approvals
		seals          mempool.Seals
		prop           *propagation.Engine
		prov           *provider.Engine
		sync           *synchronization.Engine
	)

	cmd.FlowNode("consensus").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&guaranteeLimit, "guarantee-limit", 100000, "maximum number of guarantees in the memory pool")
			flags.UintVar(&receiptLimit, "receipt-limit", 100000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&approvalLimit, "approval-limit", 100000, "maximum number of result approvals in the memory pool")
			flags.UintVar(&sealLimit, "seal-limit", 100000, "maximum number of block seals in the memory pool")
			flags.StringVarP(&chainID, "chain-id", "C", flow.DefaultChainID, "the chain ID for the protocol chain")
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
		Component("matching engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			results := storage.NewExecutionResults(node.DB)
			return matching.New(node.Logger, node.Network, node.State, node.Me, results, receipts, approvals, seals)
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			prov, err = provider.New(node.Logger, node.Metrics, node.Network, node.State, node.Me)
			return prov, err
		}).
		Component("propagation engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			prop, err = propagation.New(node.Logger, node.Network, node.State, node.Me, guarantees)
			return prop, err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ing, err := ingestion.New(node.Logger, node.Network, prop, node.State, node.Metrics, node.Me)
			return ing, err
		}).
		Component("consensus components", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// TODO: we should probably find a way to initialize mutually dependent engines separately

			// initialize the entity database accessors
			headersDB := storage.NewHeaders(node.DB)
			payloadsDB := storage.NewPayloads(node.DB)
			blocksDB := storage.NewBlocks(node.DB)
			guaranteesDB := storage.NewGuarantees(node.DB)
			sealsDB := storage.NewSeals(node.DB)

			// initialize the pending blocks cache
			cache := buffer.NewPendingBlocks()

			// initialize the compliance engine
			comp, err := compliance.New(node.Logger, node.Network, node.Me, node.State, headersDB, payloadsDB, cache)
			if err != nil {
				return nil, fmt.Errorf("could not initialize compliance engine: %w", err)
			}

			// initialize the synchronization engine
			sync, err = synchronization.New(node.Logger, node.Network, node.Me, node.State, blocksDB, comp)
			if err != nil {
				return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
			}

			// define the selector for our consensus participant set
			selector := filter.And(filter.HasRole(flow.RoleConsensus), filter.HasStake(true))

			// initialize the block builder
			build := builder.NewBuilder(node.DB, guarantees, seals,
				builder.WithChainID(chainID),
				builder.WithMinInterval(minInterval),
				builder.WithMaxInterval(maxInterval),
			)

			// initialize the block finalizer
			final := finalizer.NewFinalizer(node.DB, guarantees, seals, prov)

			// initialize the aggregating signature module for staking signatures
			staking := signature.NewAggregationProvider(encoding.ConsensusVoteTag, node.Me)

			// initialize the threshold signature module for random beacon signatures
			beacon := signature.NewThresholdProvider(encoding.RandomBeaconTag, privateDKGData.RandomBeaconPrivKey)

			// initialize the simple merger to combine staking & beacon signatures
			merger := signature.NewCombiner()

			// initialize the combined signer for hotstuff
			signer := verification.NewCombinedSigner(node.State, node.DKGState, staking, beacon, merger, selector, node.Me.NodeID())

			// initialize a logging notifier for hotstuff
			notifier := consensus.CreateNotifier(node.Logger, node.Metrics, guaranteesDB, sealsDB)

			// initialize hotstuff consensus algorithm
			hot, err := consensus.NewParticipant(
				node.Logger, notifier, node.Metrics, headersDB, node.State, node.Me, build, final, signer, comp,
				selector, &node.GenesisBlock.Header, node.GenesisQC,
				consensus.WithTimeout(hotstuffTimeout),
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize coldstuff engine: %w", err)
			}

			comp = comp.WithSynchronization(sync).WithConsensus(hot)
			return comp, nil
		}).
		Run()
}

func loadDKGPrivateData(path string, myID flow.Identifier) (*bootstrap.DKGParticipantPriv, error) {
	filename := fmt.Sprintf(bootstrap.FilenameRandomBeaconPriv, myID)
	data, err := ioutil.ReadFile(filepath.Join(path, filename))
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
