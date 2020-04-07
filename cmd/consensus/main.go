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
	"github.com/dapperlabs/flow-go/consensus/coldstuff"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/consensus/compliance"
	"github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/matching"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/consensus/provider"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	builder "github.com/dapperlabs/flow-go/module/builder/consensus"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		guaranteeLimit uint
		receiptLimit   uint
		approvalLimit  uint
		sealLimit      uint
		chainID        string
		minInterval    time.Duration
		maxInterval    time.Duration

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
		}).
		Module("random beacon key", func(node *cmd.FlowNodeBuilder) error {
			// TODO inject this into HotStuff
			privateDKGData, err = loadDKGPrivateData(node.BaseConfig.BootstrapDir, node.NodeID)
			_ = privateDKGData
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
			prov, err = provider.New(node.Logger, node.Network, node.State, node.Me)
			return prov, err
		}).
		Component("propagation engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			prop, err = propagation.New(node.Logger, node.Network, node.State, node.Me, guarantees)
			return prop, err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ing, err := ingestion.New(node.Logger, node.Network, prop, node.State, node.Me)
			return ing, err
		}).
		Component("consensus components", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// TODO: we should probably find a way to initialize mutually dependent engines separately

			headersDB := storage.NewHeaders(node.DB)
			payloadsDB := storage.NewPayloads(node.DB)
			cache := buffer.NewPendingBlocks()
			comp, err := compliance.New(node.Logger, node.Network, node.Me, node.State, headersDB, payloadsDB, cache)
			if err != nil {
				return nil, fmt.Errorf("could not initialize compliance engine: %w", err)
			}

			blocks := storage.NewBlocks(node.DB)
			sync, err = synchronization.New(node.Logger, node.Network, node.Me, node.State, blocks, comp)
			if err != nil {
				return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
			}

			memberFilter := filter.HasRole(flow.RoleConsensus)

			head := func() (*flow.Header, error) {
				return node.State.Final().Head()
			}
			build := builder.NewBuilder(node.DB, guarantees, seals,
				builder.WithChainID(chainID),
				builder.WithMinInterval(minInterval),
				builder.WithMaxInterval(maxInterval),
			)
			final := finalizer.NewFinalizer(node.DB, guarantees, seals, prov)
			cold, err := coldstuff.New(node.Logger, node.State, node.Me, comp, build, final, memberFilter, 3*time.Second, 6*time.Second, head)
			if err != nil {
				return nil, fmt.Errorf("could not initialize coldstuff engine: %w", err)
			}

			comp = comp.WithSynchronization(sync).WithConsensus(cold)
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
