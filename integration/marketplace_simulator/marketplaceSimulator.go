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

// TODO
// separate account creation and add accounts to the constructor
// inject a runnable (setup, setup accounts, run)

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
	m.txTracker, err = NewTxTracker(m.log, 10000, 10, m.networkConfig.AccessNodeAddresses[0], time.Second)
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
	m.simulatorConfig.NBATopshotAddress = accounts[0].Address
	accounts = accounts[1:]

	// setup and deploy contracts
	err = m.setupContracts()
	if err != nil {
		return err
	}

	// setup marketplace accounts
	err = m.setupMarketplaceAccounts(accounts)
	if err != nil {
		return err
	}

	// mint moments
	err = m.mintMoments()
	if err != nil {
		return err
	}

	// distribute moments

	return nil
}

func (m *MarketPlaceSimulator) setupContracts() error {

	// deploy nonFungibleContract
	err := m.deployContract("NonFungibleToken", coreContract.NonFungibleToken())
	if err != nil {
		return err
	}

	err = m.deployContract("TopShot", nbaContract.GenerateTopShotContract(m.nbaTopshotAccount.Address.Hex()))
	if err != nil {
		return err
	}

	err = m.deployContract("TopShotShardedCollection", nbaContract.GenerateTopShotShardedCollectionContract(m.nbaTopshotAccount.Address.Hex(),
		m.nbaTopshotAccount.Address.Hex()))
	if err != nil {
		return err
	}

	err = m.deployContract("TopshotAdminReceiver", nbaContract.GenerateTopshotAdminReceiverContract(m.nbaTopshotAccount.Address.Hex(),
		m.nbaTopshotAccount.Address.Hex()))
	if err != nil {
		return err
	}

	err = m.deployContract("Market", nbaContract.GenerateTopShotMarketContract(m.networkConfig.FungibleTokenAddress.Hex(),
		m.nbaTopshotAccount.Address.Hex(),
		m.nbaTopshotAccount.Address.Hex()))

	return err
}

func (m *MarketPlaceSimulator) mintMoments() error {

	nbaAddress := m.nbaTopshotAccount.Address

	// adding keys to nba account
	m.log.Info().Msgf("adding keys to the nba account %s", nbaAddress)
	err := m.nbaTopshotAccount.AddKeys(600)
	if err != nil {
		return err
	}

	// wait for the adding keys to be executed
	time.Sleep(time.Second * 10)

	// add extra keys to accounts
	// TODO move this to account setup
	wg := sync.WaitGroup{}
	for i := 0; i < len(m.marketAccounts); i++ {
		j := i
		go func() {
			wg.Add(1)
			defer wg.Done()
			m.log.Info().Msg("adding keys to accounts")
			// mint a lot of moments
			err := m.marketAccounts[j].Account().AddKeys(50)
			if err != nil {
				m.log.Error().Msgf("adding a play to a set has been failed: %w", err)
			}
		}()
		// totalMinted += batchSize
		time.Sleep(time.Millisecond * 400)
	}
	wg.Wait()

	blockRef, err := m.flowClient.GetLatestBlockHeader(context.Background(), false)
	if err != nil {
		return err
	}

	// numBuckets := 32
	// // setup nba account to use sharded collections
	// script := nbaTemplates.GenerateSetupShardedCollectionScript(*nbaAddress, *nbaAddress, numBuckets)
	// tx := flowsdk.NewTransaction().
	// 	SetReferenceBlockID(blockRef.ID).
	// 	SetScript(script)

	// result, err := m.SendTxAndWait(tx, m.nbaTopshotAccount)

	// if err != nil || result.Error != nil {
	// 	m.log.Error().Msgf("error setting up the nba account to us sharded collections: %w , %w", result.Error, err)
	// 	return err
	// }

	// TODO add many plays and add many sets (adds plays to sets)
	// this adds a play with id 0
	script := nbaTemplates.GenerateMintPlayScript(*nbaAddress, *samplePlay())
	tx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef.ID).
		SetScript(script)

	result, err := m.nbaTopshotAccount.SendTxAndWait(tx, 0)

	if err != nil {
		m.log.Error().Msgf("minting a play failed: %w", err)
		return err

	}
	if result != nil && result.Error != nil {
		m.log.Error().Msgf("minting a play failed: %w", result.Error)
		return result.Error
	}

	m.log.Info().Msgf("a play has been minted")

	// this creates set with id 0
	script = nbaTemplates.GenerateMintSetScript(*nbaAddress, "test set")
	tx = flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef.ID).
		SetScript(script)

	result, err = m.nbaTopshotAccount.SendTxAndWait(tx, 0)

	if err != nil || result.Error != nil {
		m.log.Error().Msgf("minting a set failed: %w , %w", result, err)
		return err
	}

	m.log.Info().Msgf("a set has been minted")

	script = nbaTemplates.GenerateAddPlaysToSetScript(*nbaAddress, 1, []uint32{1})
	tx = flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef.ID).
		SetScript(script)

	result, err = m.nbaTopshotAccount.SendTxAndWait(tx, 0)

	if err != nil || result.Error != nil {
		m.log.Error().Msgf("adding a play to a set has been failed: %w , %w", result, err)
		return err
	}

	m.log.Info().Msgf("play added to a set")

	batchSize := 100
	momentsPerAccount := m.simulatorConfig.NumberOfMomentsPerAccount
	steps := len(m.marketAccounts)

	wg = sync.WaitGroup{}
	for p := 0; p < momentsPerAccount/batchSize; p++ {

		blockRef, err = m.flowClient.GetLatestBlockHeader(context.Background(), false)
		if err != nil {
			return err
		}

		for i := 0; i < steps; i++ {
			j := i
			go func() {
				wg.Add(1)
				defer wg.Done()

				m.log.Info().Msgf("minting %d moments on nba account", batchSize)
				// mint a lot of moments
				script = nbaTemplates.GenerateBatchMintMomentScript(*nbaAddress, *m.marketAccounts[j].Account().Address, 1, 1, uint64(batchSize))
				tx = flowsdk.NewTransaction().
					SetReferenceBlockID(blockRef.ID).
					SetScript(script)

				result, err = m.nbaTopshotAccount.SendTxAndWait(tx, j)

				if err != nil {
					m.log.Error().Msgf("minting tx failed: %w", err)

				}
				if result != nil && result.Error != nil {
					m.log.Error().Msgf("minting tx was unsuccessful: %w", result.Error)
				}

			}()
			// totalMinted += batchSize
			time.Sleep(time.Millisecond * 300)
		}
		// this delay is needed to prevent seq number collision,
		// some how sendTxandWait doesn't wait for all transactions
		time.Sleep(time.Second * 11)
	}
	wg.Wait()

	m.log.Info().Msg("minting moments are complete")
	return nil
}

func (m *MarketPlaceSimulator) setupMarketplaceAccounts(accounts []flowAccount) error {
	// setup marketplace accounts
	// break accounts into batches of 10
	// TODO not share the same client

	groupSize := m.simulatorConfig.AccountGroupSize

	for i := 0; i < len(accounts); i += groupSize {

		blockRef, err := m.flowClient.GetLatestBlockHeader(context.Background(), false)
		if err != nil {
			return err
		}

		group := accounts[i : i+groupSize]
		// randomly select an access nodes
		n := len(m.networkConfig.AccessNodeAddresses)
		accessNode := m.networkConfig.AccessNodeAddresses[rand.Intn(n)]

		flowClient, err := client.New(accessNode, grpc.WithInsecure())
		if err != nil {
			panic(err)
		}

		txTracker, err := NewTxTracker(m.log,
			10000, // max in flight transactions
			50,    // number of workers
			accessNode,
			time.Second,
		)
		if err != nil {
			panic(err)
		}

		wg := sync.WaitGroup{}

		for _, acc := range group {
			c := acc
			ma := newMarketPlaceAccount(&c, group, m.log, txTracker, flowClient, m.simulatorConfig, accessNode)
			if ma == nil {
				panic("marketplace account was empty")
			}
			m.marketAccounts = append(m.marketAccounts, *ma)
			m.availableAccounts <- ma
			// setup account to be able to intract with nba

			m.log.Info().Msgf("setting up marketplace account with address %s", ma.Account().Address)

			go func() {
				wg.Add(1)
				defer wg.Done()
				// GenerateSetupShardedCollectionScript numBuckets 32
				numBuckets := 32
				// script := nbaTemplates.GenerateSetupAccountScript(*m.nbaTopshotAccount.Address, *m.nbaTopshotAccount.Address)

				script := nbaTemplates.GenerateSetupShardedCollectionScript(*m.nbaTopshotAccount.Address, *m.nbaTopshotAccount.Address, numBuckets)

				tx := flowsdk.NewTransaction().
					SetReferenceBlockID(blockRef.ID).
					SetScript(script)

				result, err := ma.Account().SendTxAndWait(tx, 0)

				if err != nil || result.Error != nil {
					m.log.Error().Msgf("setting up marketplace accounts failed: %w , %w", result, err)
				}
				m.log.Debug().Msg("account setup is done")

			}()

			// // transfer some moments
			// moments := makeMomentRange(momentCounter, momentCounter+20)
			// momentCounter += 20
			// script = generateBatchTransferMomentScript(m.nbaTopshotAccount.Address, m.nbaTopshotAccount.Address, ma.Account().Address, moments)
			// // script = generateBatchTransferMomentfromShardedCollectionScript(m.nbaTopshotAccount.Address, m.nbaTopshotAccount.Address, m.nbaTopshotAccount.Address, ma.Account().Address, moments)

			// tx = flowsdk.NewTransaction().
			// 	SetReferenceBlockID(blockRef.ID).
			// 	SetScript(script)

			// result, err = m.SendTxAndWait(tx, m.nbaTopshotAccount)
			// if err != nil || result.Error != nil {
			// 	m.log.Error().Msgf("transfering initial moments to a marketplace account failed: %s , %w", result, err)
			// 	return err
			// }

			// m.log.Debug().Msg("transferring moments are done")

			// setup sales
			// GenerateCreateSaleScript(m.nbaTopshotAccount.Address, ma.Account().Address, tokenStorageName string, 0.15)
			// nbaTemplate.GenerateCreateSaleScript()

			// get moments
			// ma.GetMoments()
		}
		wg.Wait()
	}

	return nil
}

func (m *MarketPlaceSimulator) deployContract(name string, contract []byte) error {
	blockRef, err := m.flowClient.GetLatestBlockHeader(context.Background(), false)
	if err != nil {
		return err
	}

	template := `
	transaction {
		prepare(signer: AuthAccount) {
			signer.contracts.add(name: "%s",
			                     code: "%s".decodeHex())
		}
	}
	`

	script := []byte(fmt.Sprintf(template, name, hex.EncodeToString([]byte(contract))))

	deploymentTx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef.ID).
		SetScript(script)

	result, err := m.nbaTopshotAccount.SendTxAndWait(deploymentTx, 0)

	if err != nil || result.Error != nil {
		m.log.Error().Msgf("contract %s deployment is failed : %w , %w", name, result, err)
	}

	m.log.Info().Msgf("contract %s is deployed : %s", name, result)
	return err
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
			*serviceAcc.Address,
			serviceAcc.accountKeys[0].Index,
			serviceAcc.accountKeys[0].SequenceNumber,
		).
		AddAuthorizer(*serviceAcc.Address).
		SetPayer(*serviceAcc.Address)

	publicKey := bytesToCadenceArray(accountKey.Encode())
	count := cadence.NewInt(num)

	initialTokenAmount, err := cadence.NewUFix64FromParts(
		24*60*60*0.01,
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
		*serviceAcc.Address,
		serviceAcc.accountKeys[0].Index,
		serviceAcc.signer,
	)
	if err != nil {
		return nil, err
	}
	serviceAcc.accountKeys[0].SequenceNumber++
	serviceAcc.signerLock.Unlock()

	err = m.flowClient.SendTransaction(context.Background(), *createAccountTx)
	if err != nil {
		return nil, err
	}

	wg.Add(1)

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

					newAccKey := flowsdk.NewAccountKey().
						FromPrivateKey(privKey).
						SetHashAlgo(crypto.SHA3_256).
						SetWeight(flowsdk.AccountKeyWeightThreshold)

					newAcc, err := newFlowAccount(&accountAddress, hex.EncodeToString(privKey.Encode()), []*flowsdk.AccountKey{newAccKey}, signer, m.flowClient, m.txTracker, m.log)
					if err != nil {
						panic(err)
					}
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

func (m *MarketPlaceSimulator) Run() error {

	// select an account
	// call Act and put it back to list when is returned

	for i := 0; i < len(m.marketAccounts); i++ {
		j := i
		go func() {
			actor := m.marketAccounts[j]
			fmt.Println("running account :", actor.Account().Address.String())
			err := actor.Act()
			fmt.Println("err: ", err)
			// TODO handle the retuned error
		}()
		// for better distribution of load
		time.Sleep(time.Millisecond * 100)
	}

	return nil
}

type marketPlaceAccount struct {
	log             zerolog.Logger
	account         *flowAccount
	friends         []flowAccount
	flowClient      *client.Client
	txTracker       *TxTracker
	simulatorConfig *SimulatorConfig
}

func newMarketPlaceAccount(account *flowAccount,
	friends []flowAccount,
	log zerolog.Logger,
	txTracker *TxTracker,
	flowClient *client.Client,
	simulatorConfig *SimulatorConfig,
	accessNodeAddr string) *marketPlaceAccount {

	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator

	return &marketPlaceAccount{
		log:             log,
		account:         account,
		friends:         friends,
		txTracker:       txTracker,
		flowClient:      flowClient,
		simulatorConfig: simulatorConfig,
	}
}

func (m *marketPlaceAccount) Account() *flowAccount {
	return m.account
}

func (m *marketPlaceAccount) GetMoments() ([]uint64, error) {

	blockRef, err := m.flowClient.GetLatestBlockHeader(context.Background(), false)
	if err != nil {
		return nil, err
	}

	template := `
	import TopShot from 0x%s

	pub fun main(): [UInt64] {

		let acct = getAccount(0x%s)

		let collectionRef = acct.getCapability(/public/MomentCollection).borrow<&{TopShot.MomentCollectionPublic}>()!

		return collectionRef.getIDs()
	}
	`

	script := []byte(fmt.Sprintf(template, m.simulatorConfig.NBATopshotAddress.String(), m.account.Address.String()))

	res, err := m.flowClient.ExecuteScriptAtBlockID(context.Background(), blockRef.ID, script, nil)

	if err != nil {
		return nil, err
	}
	result := make([]uint64, 0)
	v := res.ToGoValue().([]interface{})
	for _, i := range v {
		result = append(result, i.(uint64))
	}

	return result, err
}

func (m *marketPlaceAccount) Act() error {

	// with some chance don't do anything

	// list a moment to sell

	// query for active listings to buy

	// // randomly select one or two friend and send assets
	// assets := m.GetAssets()

	// assetToMove := assets[rand.Intn(len(assets))]

	// _ = assetToMove

	duration := time.Minute * 20
	for start := time.Now(); ; {
		if time.Since(start) > duration {
			break
		}

		blockRef, err := m.flowClient.GetLatestBlockHeader(context.Background(), false)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		moments, err := m.GetMoments()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if len(moments) == 0 {
			time.Sleep(time.Second)
			continue
		}

		// send to all friends for now
		// TODO set a ratio
		numberOfTx := m.simulatorConfig.AccountGroupSize
		// f := rand.Intn(len(m.friends) - numberOfTx)
		// friends := m.friends[f : f+numberOfTx]
		friends := m.friends[:]

		transferSize := numberOfTx * m.simulatorConfig.MomentsToTransferPerTx
		r := rand.Intn(len(moments) - transferSize)
		selected := moments[r : r+transferSize]
		share := m.simulatorConfig.MomentsToTransferPerTx

		wg := sync.WaitGroup{}
		for j := 0; j < len(friends); j++ {
			p := j
			go func() {
				wg.Add(1)
				defer wg.Done()

				txScript := generateBatchTransferMomentfromShardedCollectionScript(m.simulatorConfig.NBATopshotAddress,
					m.simulatorConfig.NBATopshotAddress,
					m.simulatorConfig.NBATopshotAddress,
					friends[p].Address,
					selected[p*share:p*share+share])

				tx := flowsdk.NewTransaction().
					SetReferenceBlockID(blockRef.ID).
					SetScript(txScript)

				result, err := m.Account().SendTxAndWait(tx, p)
				if err != nil || result.Error != nil {
					m.log.Error().Msgf("marketplace tx failed: %w , %w", result.Error, err)
				}
			}()
		}
		wg.Wait()
		time.Sleep(time.Second * 10)
	}

	return nil
}
