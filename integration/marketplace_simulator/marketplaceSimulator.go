package marketplace

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"time"

	nbaContract "github.com/dapperlabs/nba-smart-contracts/lib/go/contracts"
	nbaTemplates "github.com/dapperlabs/nba-smart-contracts/lib/go/templates"
	nbaData "github.com/dapperlabs/nba-smart-contracts/lib/go/templates/data"
	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	coreContract "github.com/onflow/flow-nft/lib/go/contracts"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
)

// MarketPlaceSimulator simulates continuous variable load with interactions
type MarketPlaceSimulator struct {
	log               zerolog.Logger
	networkConfig     *NetworkConfig
	simulatorConfig   *SimulatorConfig
	nbaTopshotAccount *flowAccount
	marketAccounts    []marketPlaceAccount
	availableAccounts chan *marketPlaceAccount
	stopped           bool
	flowClient        *client.Client
	txTracker         *TxTracker
}

func NewMarketPlaceSimulator(
	log zerolog.Logger,
	networkConfig *NetworkConfig,
	simulatorConfig *SimulatorConfig,
) *MarketPlaceSimulator {
	sim := &MarketPlaceSimulator{
		log:               log,
		networkConfig:     networkConfig,
		simulatorConfig:   simulatorConfig,
		marketAccounts:    make([]marketPlaceAccount, 0),
		availableAccounts: make(chan *marketPlaceAccount, simulatorConfig.NumberOfAccounts),
		stopped:           false,
	}

	err := sim.Setup()
	if err != nil {
		panic(err)
	}
	return sim
}

func (m *MarketPlaceSimulator) Setup() error {

	var err error
	// setup client
	m.flowClient, err = client.New(m.networkConfig.AccessNodeAddresses[0], grpc.WithInsecure())
	if err != nil {
		return nil
	}

	// setup tracker (TODO simplify this by using default values and empty txStatsTracker)
	m.txTracker, err = NewTxTracker(m.log, 1000, 10, m.networkConfig.AccessNodeAddresses[0], 1)
	if err != nil {
		return nil
	}

	// load service account
	serviceAcc, err := loadServiceAccount(m.flowClient,
		m.networkConfig.ServiceAccountAddress,
		m.networkConfig.ServiceAccountPrivateKeyHex)
	if err != nil {
		return fmt.Errorf("error loading service account %w", err)
	}

	accounts, err := m.createAccounts(serviceAcc, m.simulatorConfig.NumberOfAccounts+1) // first one is for nba
	if err != nil {
		return err
	}

	// set the nbatopshot account first
	m.nbaTopshotAccount = &accounts[0]
	accounts = accounts[1:]

	// setup and deploy contracts
	err = m.setupContracts()
	if err != nil {
		return err
	}

	// mint moments
	err = m.mintMoments()
	if err != nil {
		return err
	}

	// setup marketplace accounts
	err = m.setupMarketplaceAccounts(accounts)
	if err != nil {
		return err
	}

	// distribute moments

	return nil
}

func (m *MarketPlaceSimulator) setupContracts() error {

	// deploy nonFungibleContract
	err := m.deployContract(coreContract.NonFungibleToken())
	if err != nil {
		return err
	}

	err = m.deployContract(nbaContract.GenerateTopShotContract(m.nbaTopshotAccount.Address().Hex()))
	if err != nil {
		return err
	}

	err = m.deployContract(nbaContract.GenerateTopShotShardedCollectionContract(m.nbaTopshotAccount.Address().Hex(),
		m.nbaTopshotAccount.Address().Hex()))
	if err != nil {
		return err
	}

	err = m.deployContract(nbaContract.GenerateTopshotAdminReceiverContract(m.nbaTopshotAccount.Address().Hex(),
		m.nbaTopshotAccount.Address().Hex()))
	if err != nil {
		return err
	}

	err = m.deployContract(nbaContract.GenerateTopShotMarketContract(m.networkConfig.FungibleTokenAddress.Hex(),
		m.nbaTopshotAccount.Address().Hex(),
		m.nbaTopshotAccount.Address().Hex(),
		m.nbaTopshotAccount.Address().Hex()))

	return err
}

func (m *MarketPlaceSimulator) mintMoments() error {
	blockRef, err := m.flowClient.GetLatestBlockHeader(context.Background(), false)
	if err != nil {
		return err
	}

	nbaAddress := m.nbaTopshotAccount.Address()
	// TODO add many plays and add many sets (adds plays to sets)
	// this adds a play with id 0
	script := nbaTemplates.GenerateMintPlayScript(*nbaAddress, *samplePlay())
	tx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef.ID).
		SetScript(script)

	m.sendTxAndWait(tx, m.nbaTopshotAccount)

	// this creates set with id 0
	script = nbaTemplates.GenerateMintSetScript(*nbaAddress, "test set")
	tx = flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef.ID).
		SetScript(script)

	m.sendTxAndWait(tx, m.nbaTopshotAccount)

	script = nbaTemplates.GenerateAddPlaysToSetScript(*nbaAddress, 0, []uint32{0})
	tx = flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef.ID).
		SetScript(script)

	m.sendTxAndWait(tx, m.nbaTopshotAccount)

	// mint a lot of moments
	// GenerateBatchMintMomentScript(topShotAddr flow.Address, destinationAccount flow.Address, setID, playID uint32, quantity uint64)
	script = nbaTemplates.GenerateBatchMintMomentScript(*nbaAddress, *nbaAddress, 0, 0, uint64(m.simulatorConfig.NumberOfMoments))
	tx = flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef.ID).
		SetScript(script)

	m.sendTxAndWait(tx, m.nbaTopshotAccount)
	return nil
}

func (m *MarketPlaceSimulator) setupMarketplaceAccounts(accounts []flowAccount) error {
	// setup marketplace accounts
	// break accounts into batches of 10
	// TODO not share the same client
	groupSize := 10
	for i := 0; i < len(accounts); i += groupSize {
		group := accounts[i : i+groupSize]
		// randomly select an access nodes
		n := len(m.networkConfig.AccessNodeAddresses)
		accessNode := m.networkConfig.AccessNodeAddresses[rand.Intn(n)]

		for _, acc := range group {
			ma := newMarketPlaceAccount(&acc, group, m.log, accessNode)
			m.marketAccounts = append(m.marketAccounts, *ma)
			m.availableAccounts <- ma
			// setup account to be able to intract with nba
			nbaTemplates.GenerateSetupAccountScript(*m.nbaTopshotAccount.Address(), *m.nbaTopshotAccount.Address())

		}
	}

	// TODO transfer some moments

	return nil
}

func (m *MarketPlaceSimulator) Run() error {

	// acc := <-lg.availableAccounts
	// defer func() { lg.availableAccounts <- acc }()

	// select a random account
	// call Act and put it back to list when is returned
	// go Run (wrap func into a one to return the account back to list)
	return nil
}

func (m *MarketPlaceSimulator) deployContract(contract []byte) error {
	blockRef, err := m.flowClient.GetLatestBlockHeader(context.Background(), false)
	if err != nil {
		return err
	}

	template := `
	transaction {
		prepare(signer: AuthAccount) {
			signer.setCode("%s".decodeHex())
		}
	}
	`
	script := []byte(fmt.Sprintf(template, hex.EncodeToString([]byte(contract))))

	deploymentTx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef.ID).
		SetScript(script)

	m.sendTxAndWait(deploymentTx, m.nbaTopshotAccount)
	return nil
}

// TODO update this to support multiple tx submissions
func (m *MarketPlaceSimulator) sendTxAndWait(tx *flowsdk.Transaction, sender *flowAccount) {

	err := sender.PrepareAndSignTx(tx, 0)
	if err != nil {
		m.log.Error().Err(err).Msg("error preparing and signing the transaction")
		return
	}

	err = m.flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		m.log.Error().Err(err).Msg("error sending the transaction")
		return
	}
	stopped := false
	wg := sync.WaitGroup{}
	m.txTracker.AddTx(tx.ID(),
		nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			m.log.Trace().Str("tx_id", tx.ID().String()).Msgf("finalized tx")
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on finalized
		func(_ flowsdk.Identifier, _ *flowsdk.TransactionResult) {
			m.log.Trace().Str("tx_id", tx.ID().String()).Msgf("sealed tx")
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
}

func (m *MarketPlaceSimulator) createAccounts(serviceAcc *flowAccount, num int) ([]flowAccount, error) {
	m.log.Info().Msgf("creating and funding %d accounts...", num)

	accounts := make([]flowAccount, 0)

	blockRef, err := m.flowClient.GetLatestBlockHeader(context.Background(), false)
	if err != nil {
		return nil, err
	}

	wg := sync.WaitGroup{}

	privKey := randomPrivateKey()
	accountKey := flowsdk.NewAccountKey().
		FromPrivateKey(privKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flowsdk.AccountKeyWeightThreshold)

	// Generate an account creation script
	createAccountTx := flowsdk.NewTransaction().
		SetScript(createAccountsScript(*m.networkConfig.FungibleTokenAddress,
			*m.networkConfig.FlowTokenAddress)).
		SetReferenceBlockID(blockRef.ID).
		SetProposalKey(
			*serviceAcc.address,
			serviceAcc.accountKey.Index,
			serviceAcc.accountKey.SequenceNumber,
		).
		AddAuthorizer(*serviceAcc.address).
		SetPayer(*serviceAcc.address)

	publicKey := bytesToCadenceArray(accountKey.Encode())
	count := cadence.NewInt(num)

	initialTokenAmount, err := cadence.NewUFix64FromParts(
		24*60*60,
		0,
	)
	if err != nil {
		return nil, err
	}

	err = createAccountTx.AddArgument(publicKey)
	if err != nil {
		return nil, err
	}

	err = createAccountTx.AddArgument(count)
	if err != nil {
		return nil, err
	}

	err = createAccountTx.AddArgument(initialTokenAmount)
	if err != nil {
		return nil, err
	}

	// TODO replace with account.Sign
	serviceAcc.signerLock.Lock()
	err = createAccountTx.SignEnvelope(
		*serviceAcc.address,
		serviceAcc.accountKey.Index,
		serviceAcc.signer,
	)
	if err != nil {
		return nil, err
	}
	serviceAcc.accountKey.SequenceNumber++
	serviceAcc.signerLock.Unlock()

	err = m.flowClient.SendTransaction(context.Background(), *createAccountTx)
	if err != nil {
		return nil, err
	}

	wg.Add(1)

	i := 0

	m.txTracker.AddTx(createAccountTx.ID(),
		nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			defer wg.Done()

			m.log.Debug().
				Str("status", res.Status.String()).
				Msg("account creation tx executed")

			if res.Error != nil {
				m.log.Error().
					Err(res.Error).
					Msg("account creation tx failed")
			}

			for _, event := range res.Events {
				m.log.Trace().
					Str("event_type", event.Type).
					Str("event", event.String()).
					Msg("account creation tx event")

				if event.Type == flowsdk.EventAccountCreated {
					accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
					accountAddress := accountCreatedEvent.Address()

					m.log.Debug().
						Hex("address", accountAddress.Bytes()).
						Msg("new account created")

					signer := crypto.NewInMemorySigner(privKey, accountKey.HashAlgo)

					newAcc := newFlowAccount(i, &accountAddress, accountKey, signer)
					i++

					accounts = append(accounts, *newAcc)

					m.log.Debug().
						Hex("address", accountAddress.Bytes()).
						Msg("new account added")
				}
			}
		},
		nil, // on sealed
		func(_ flowsdk.Identifier) {
			m.log.Error().Msg("setup transaction (account creation) has expired")
			wg.Done()
		}, // on expired
		func(_ flowsdk.Identifier) {
			m.log.Error().Msg("setup transaction (account creation) has timed out")
			wg.Done()
		}, // on timeout
		func(_ flowsdk.Identifier, err error) {
			m.log.Error().Err(err).Msg("setup transaction (account creation) encountered an error")
			wg.Done()
		}, // on error
		120)

	wg.Wait()

	m.log.Info().Msgf("created %d accounts", len(accounts))

	return accounts, nil
}

type marketPlaceAccount struct {
	log        zerolog.Logger
	account    *flowAccount
	friends    []flowAccount
	flowClient *client.Client
	txTracker  *TxTracker
}

func newMarketPlaceAccount(account *flowAccount,
	friends []flowAccount,
	log zerolog.Logger,
	accessNodeAddr string) *marketPlaceAccount {
	txTracker, err := NewTxTracker(log,
		10, // max in flight transactions
		1,  // number of workers
		accessNodeAddr,
		1, // number of accounts
	)
	if err != nil {
		panic(err)
	}
	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator

	fclient, err := client.New(accessNodeAddr, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	return &marketPlaceAccount{
		log:        log,
		account:    account,
		friends:    friends,
		txTracker:  txTracker,
		flowClient: fclient,
	}
}

func (m *marketPlaceAccount) GetAssets() []uint {
	// m.flowClient.Script()
	return nil
}

func (m *marketPlaceAccount) Act() {

	// nbaTemplates.GenerateTransferMomentScript(nbaTopshotAddress, nbaTopshotAddress, recipientAddr flow.Address, tokenID int)

	// with some chance don't do anything

	// // randomly select one or two friend and send assets
	// assets := m.GetAssets()

	// assetToMove := assets[rand.Intn(len(assets))]

	// _ = assetToMove

	// // TODO txScript for assetToMove
	// txScript := []byte("")

	// tx := flowsdk.NewTransaction().
	// 	SetReferenceBlockID(blockRef).
	// 	SetScript(txScript).
	// 	SetProposalKey(*m.account.address, 0, m.account.seqNumber).
	// 	SetPayer(*m.account.address).
	// 	AddAuthorizer(*m.account.address)

	// err = m.account.signTx(tx, 0)
	// if err != nil {
	// 	m.log.Error().Err(err).Msgf("error signing transaction")
	// 	return
	// }

	// // wait till success and then update the list
	// // send tx
	// err = m.flowClient.SendTransaction(context.Background(), *tx)
	// if err != nil {
	// 	m.log.Error().Err(err).Msgf("error sending transaction")
	// 	return
	// }

	// // tracking
	// stopped := false
	// wg := sync.WaitGroup{}
	// m.txTracker.AddTx(tx.ID(),
	// 	nil,
	// 	func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
	// 		m.log.Trace().Str("tx_id", tx.ID().String()).Msgf("finalized tx")
	// 	}, // on finalized
	// 	func(_ flowsdk.Identifier, _ *flowsdk.TransactionResult) {
	// 		m.log.Trace().Str("tx_id", tx.ID().String()).Msgf("sealed tx")
	// 		if !stopped {
	// 			stopped = true
	// 			wg.Done()
	// 		}
	// 	}, // on sealed
	// 	func(_ flowsdk.Identifier) {
	// 		m.log.Warn().Str("tx_id", tx.ID().String()).Msgf("tx expired")
	// 		if !stopped {
	// 			stopped = true
	// 			wg.Done()
	// 		}
	// 	}, // on expired
	// 	func(_ flowsdk.Identifier) {
	// 		m.log.Warn().Str("tx_id", tx.ID().String()).Msgf("tx timed out")
	// 		if !stopped {
	// 			stopped = true
	// 			wg.Done()
	// 		}
	// 	}, // on timout
	// 	func(_ flowsdk.Identifier, err error) {
	// 		m.log.Error().Err(err).Str("tx_id", tx.ID().String()).Msgf("tx error")
	// 		if !stopped {
	// 			stopped = true
	// 			wg.Done()
	// 		}
	// 	}, // on error
	// 	60)
	// wg.Add(1)
	// wg.Wait()

	// return
}

func samplePlay() *nbaData.PlayMetadata {
	return &nbaData.PlayMetadata{
		FullName:             "Ben Simmons",
		FirstName:            "Ben",
		LastName:             "Simmons",
		Birthdate:            "1996-07-20",
		Birthplace:           "Melbourne,, AUS",
		JerseyNumber:         "25",
		DraftTeam:            "Philadelphia 76ers",
		TeamAtMomentNBAID:    "1610612755",
		TeamAtMoment:         "Philadelphia 76ers",
		PrimaryPosition:      "PG",
		PlayerPosition:       "G",
		TotalYearsExperience: "2",
		NbaSeason:            "2019-20",
		DateOfMoment:         "2019-12-29T01:00:00Z",
		PlayCategory:         "Jump Shot",
		PlayType:             "2 Pointer",
		HomeTeamName:         "Miami Heat",
		AwayTeamName:         "Philadelphia 76ers",
	}
}
