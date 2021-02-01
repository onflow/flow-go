package utils

import (
	"context"
	"math/rand"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/metrics"

	"github.com/onflow/flow-go-sdk/client"
)

// MarketPlaceSimulator simulates continuous variable load with interactions
type MarketPlaceSimulator struct {
	log                  zerolog.Logger
	loaderMetrics        *metrics.LoaderCollector
	initialized          bool
	tps                  int
	numberOfAccounts     int
	flowClient           *client.Client
	supervisorClient     *client.Client
	serviceAccount       *flowAccount
	flowTokenAddress     *flowsdk.Address
	fungibleTokenAddress *flowsdk.Address
	favContractAddress   *flowsdk.Address
	marketAccounts       []*marketPlaceAccount
	availableAccounts    chan *marketPlaceAccount
	stopped              bool
}

func (m *MarketPlaceSimulator) Setup() error {

	// create accounts, construct markets (set friends)
	// deploy the contracts
	// distribute assets
	return nil
}

func (m *MarketPlaceSimulator) Run() error {
	// select a random account
	// call Act and put it back to list when is returned
	return nil
}

type marketPlaceAccount struct {
	log        zerolog.Logger
	account    *flowAccount
	friends    []flowsdk.Address
	blockRef   *BlockRef
	flowClient *client.Client
	txTracker  *TxTracker
}

func newMarketPlaceAccount(account *flowAccount,
	friends []flowsdk.Address,
	log zerolog.Logger,
	client *client.Client,
	loadedAccessAddr string) *marketPlaceAccount {
	txTracker, err := NewTxTracker(log,
		10, // max in flight transactions
		2,  // number of workers
		loadedAccessAddr,
		1, // number of accounts
		NewTxStatsTracker(&StatsConfig{}))
	if err != nil {
		panic(err)
	}
	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator

	return &marketPlaceAccount{
		log:        log,
		account:    account,
		friends:    friends,
		txTracker:  txTracker,
		flowClient: client,
		blockRef:   NewBlockRef(client),
	}
}

func (m *marketPlaceAccount) GetAssets() []uint {
	// m.flowClient.Script()
	return nil
}

func (m *marketPlaceAccount) Act() {
	// with some chance don't do anything

	// randomly select one or two friend and send assets
	assets := m.GetAssets()

	assetToMove := assets[rand.Intn(len(assets))]

	_ = assetToMove

	blockRef, err := m.blockRef.Get()
	if err != nil {
		m.log.Error().Err(err).Msgf("error getting reference block")
		return
	}

	// TODO txScript for assetToMove
	txScript := []byte("")

	tx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef).
		SetScript(txScript).
		SetProposalKey(*m.account.address, 0, m.account.seqNumber).
		SetPayer(*m.account.address).
		AddAuthorizer(*m.account.address)

	err = m.account.signTx(tx, 0)
	if err != nil {
		m.log.Error().Err(err).Msgf("error signing transaction")
		return
	}

	// wait till success and then update the list
	// send tx
	err = m.flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		m.log.Error().Err(err).Msgf("error sending transaction")
		return
	}

	// tracking
	stopped := false
	wg := sync.WaitGroup{}
	m.txTracker.AddTx(tx.ID(),
		nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			m.log.Trace().Str("tx_id", tx.ID().String()).Msgf("finalized tx")
		}, // on finalized
		func(_ flowsdk.Identifier, _ *flowsdk.TransactionResult) {
			m.log.Trace().Str("tx_id", tx.ID().String()).Msgf("sealed tx")
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on sealed
		func(_ flowsdk.Identifier) {
			m.log.Warn().Str("tx_id", tx.ID().String()).Msgf("tx expired")
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on expired
		func(_ flowsdk.Identifier) {
			m.log.Warn().Str("tx_id", tx.ID().String()).Msgf("tx timed out")
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on timout
		func(_ flowsdk.Identifier, err error) {
			m.log.Error().Err(err).Str("tx_id", tx.ID().String()).Msgf("tx error")
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on error
		60)
	wg.Add(1)
	wg.Wait()

	return
}
