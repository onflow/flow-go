package dkg

import (
	"fmt"
	"github.com/onflow/flow-go/module"
	"os"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	emulator "github.com/onflow/flow-emulator"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"
	dkgeng "github.com/onflow/flow-go/engine/consensus/dkg"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/dkg"
	emulatormod "github.com/onflow/flow-go/module/emulator"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

const numberOfNodes = 10

type DKGSuite struct {
	suite.Suite

	chainID                flow.ChainID
	hub                    *stub.Hub // in-mem test network
	env                    templates.Environment
	blockchain             *emulator.Blockchain
	adminEmulatorClient    *emulatormod.EmulatorClient
	adminDKGContractClient *dkg.Client
	dkgAddress             sdk.Address
	dkgAccountKey          *sdk.AccountKey
	dkgSigner              sdkcrypto.Signer
	checkDKGUnhappy        bool // activate log hook for DKGBroker to check if the DKG core is flagging misbehaviours

	netIDs       flow.IdentityList
	nodeAccounts []*nodeAccount
	nodes        []*node
}

func (s *DKGSuite) SetupTest() {
	s.initEmulator()
	s.deployDKGContract()
	s.setupDKGAdmin()

	s.netIDs = unittest.IdentityListFixture(numberOfNodes, unittest.WithRole(flow.RoleConsensus))
	for _, id := range s.netIDs {
		s.nodeAccounts = append(s.nodeAccounts, s.createAndFundAccount(id))
	}

	for _, acc := range s.nodeAccounts {
		s.nodes = append(s.nodes, s.createNode(acc))
	}

	s.startDKGWithParticipants(s.nodeAccounts)

	for _, node := range s.nodes {
		s.claimDKGParticipant(node)
	}
}

func (s *DKGSuite) BeforeTest(suiteName, testName string) {
	// In the happy case we add a log hook to check if the DKGBroker emits Warn
	// logs (which it shouldn't)
	if testName == "TestHappyPath" {
		s.checkDKGUnhappy = true
	}
	// We need to initialise the nodes with a list of identities that contain
	// all roles, otherwise there would be an error initialising the first epoch
	identities := unittest.CompleteIdentitySet(s.netIDs...)
	for _, node := range s.nodes {
		s.initEngines(node, identities)
	}
}

func (s *DKGSuite) TearDownTest() {
	s.hub = nil
	s.blockchain = nil
	s.adminEmulatorClient = nil
	s.adminDKGContractClient = nil
	s.netIDs = nil
	s.nodeAccounts = []*nodeAccount{}
	s.nodes = []*node{}
	s.checkDKGUnhappy = false
}

// initEmulator initializes the emulator and the admin emulator client
func (s *DKGSuite) initEmulator() {
	s.chainID = flow.Emulator

	blockchain, err := emulator.NewBlockchain(
		emulator.WithTransactionExpiry(flow.DefaultTransactionExpiry),
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(s.T(), err)

	s.blockchain = blockchain

	s.adminEmulatorClient = emulatormod.NewEmulatorClient(blockchain)

	s.hub = stub.NewNetworkHub()
}

// deployDKGContract deploys the DKG contract to the emulator and initializes
// the admin DKG contract client
func (s *DKGSuite) deployDKGContract() {
	// create new account keys for the DKG contract
	dkgAccountKey, dkgAccountSigner := test.AccountKeyGenerator().NewWithSigner()

	// deploy the contract to the emulator
	dkgAddress, err := s.blockchain.CreateAccount([]*sdk.AccountKey{dkgAccountKey}, []sdktemplates.Contract{
		{
			Name:   "FlowDKG",
			Source: string(contracts.FlowDKG()),
		},
	})
	require.NoError(s.T(), err)

	env := templates.Environment{
		DkgAddress: dkgAddress.Hex(),
	}

	s.env = env
	s.dkgAddress = dkgAddress
	s.dkgAccountKey = dkgAccountKey
	s.dkgSigner = dkgAccountSigner

	s.adminDKGContractClient = dkg.NewClient(
		zerolog.Nop(),
		s.adminEmulatorClient,
		s.dkgSigner,
		s.dkgAddress.String(),
		s.dkgAddress.String(), 0)
}

func (s *DKGSuite) setupDKGAdmin() {
	setUpAdminTx := sdk.NewTransaction().
		SetScript(templates.GeneratePublishDKGParticipantScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(
			s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.dkgAddress)
	_, err := s.prepareAndSubmit(setUpAdminTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.dkgSigner},
	)
	require.NoError(s.T(), err)
}

// createAndFundAccount creates a nodeAccount and funds it in the emulator
func (s *DKGSuite) createAndFundAccount(netID *flow.Identity) *nodeAccount {
	accountPrivateKey := common.RandomPrivateKey()
	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetSigAlgo(sdkcrypto.ECDSA_P256).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)
	accountID := accountKey.PublicKey.String()
	accountSigner := sdkcrypto.NewInMemorySigner(accountPrivateKey, accountKey.HashAlgo)

	/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	create Flow account
	~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

	newAccountAddress, err := s.blockchain.CreateAccount(
		[]*sdk.AccountKey{accountKey},
		[]sdktemplates.Contract{},
	)
	require.NoError(s.T(), err)

	/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	fund Flow account
	~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

	fundAccountTx := sdk.NewTransaction().
		SetScript(
			[]byte(
				fmt.Sprintf(`
				import FungibleToken from 0x%s
				import FlowToken from 0x%s
	
				transaction(amount: UFix64, recipient: Address) {
				  let sentVault: @FungibleToken.Vault
				  prepare(signer: AuthAccount) {
					let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
					  ?? panic("failed to borrow reference to sender vault")
					self.sentVault <- vaultRef.withdraw(amount: amount)
				  }
				  execute {
					let receiverRef =  getAccount(recipient)
					  .getCapability(/public/flowTokenReceiver)
					  .borrow<&{FungibleToken.Receiver}>()
						?? panic("failed to borrow reference to recipient vault")
					receiverRef.deposit(from: <-self.sentVault)
				  }
				}`,
					fvm.FungibleTokenAddress(s.chainID.Chain()).Hex(),
					fvm.FlowTokenAddress(s.chainID.Chain()).Hex(),
				))).
		AddAuthorizer(s.blockchain.ServiceKey().Address).
		SetProposalKey(
			s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber,
		).
		SetPayer(s.blockchain.ServiceKey().Address)

	err = fundAccountTx.AddArgument(cadence.UFix64(1_000_000))
	require.NoError(s.T(), err)
	err = fundAccountTx.AddArgument(cadence.NewAddress(newAccountAddress))
	require.NoError(s.T(), err)

	_, err = s.prepareAndSubmit(fundAccountTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer()},
	)
	require.NoError(s.T(), err)

	/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	create nodeAccount
	~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

	account := &nodeAccount{
		netID:          netID,
		privKey:        accountPrivateKey,
		accountKey:     accountKey,
		accountID:      accountID,
		accountAddress: newAccountAddress,
		accountSigner:  accountSigner,
		accountInfo: &bootstrap.NodeMachineAccountInfo{
			Address:           newAccountAddress.String(),
			EncodedPrivateKey: accountPrivateKey.Encode(),
			KeyIndex:          0,
			SigningAlgorithm:  accountKey.SigAlgo,
			HashAlgorithm:     accountKey.HashAlgo,
		},
	}

	return account
}

// createNode creates a DKG test node from an account and initializes its DKG
// smart-contract client
func (s *DKGSuite) createNode(account *nodeAccount) *node {
	emulatorClient := emulatormod.NewEmulatorClient(s.blockchain)
	contractClient := dkg.NewClient(
		zerolog.Nop(),
		emulatorClient,
		account.accountSigner,
		s.dkgAddress.String(),
		account.accountAddress.String(),
		0,
	)
	dkgClientWrapper := NewDKGClientWrapper(contractClient)
	return &node{
		account:           account,
		dkgContractClient: dkgClientWrapper,
	}
}

func (s *DKGSuite) startDKGWithParticipants(accounts []*nodeAccount) {
	// convert node identifiers to candece.Value to be passed in as TX argument
	valueNodeIDs := make([]cadence.Value, 0, len(accounts))
	for _, account := range accounts {
		valueAccountID, err := cadence.NewString(account.accountID)
		s.Require().NoError(err)
		valueNodeIDs = append(valueNodeIDs, valueAccountID)
	}

	// start DKG using admin resource
	startDKGTx := sdk.NewTransaction().
		SetScript(templates.GenerateStartDKGScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(
			s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.dkgAddress)

	err := startDKGTx.AddArgument(cadence.NewArray(valueNodeIDs))
	require.NoError(s.T(), err)

	_, err = s.prepareAndSubmit(startDKGTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.dkgSigner},
	)
	require.NoError(s.T(), err)

	// sanity check: verify that DKG was started with correct node IDs
	result := s.executeScript(templates.GenerateGetConsensusNodesScript(s.env), nil)
	assert.Equal(s.T(), cadence.NewArray(valueNodeIDs), result)
}

func (s *DKGSuite) claimDKGParticipant(node *node) {
	createParticipantTx := sdk.NewTransaction().
		SetScript(templates.GenerateCreateDKGParticipantScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(
			s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber,
		).
		SetPayer(node.account.accountAddress).
		AddAuthorizer(node.account.accountAddress)

	err := createParticipantTx.AddArgument(cadence.NewAddress(s.dkgAddress))
	require.NoError(s.T(), err)
	valueAccountPubKey, err := cadence.NewString(node.account.accountKey.PublicKey.String())
	require.NoError(s.T(), err)
	err = createParticipantTx.AddArgument(valueAccountPubKey)
	require.NoError(s.T(), err)

	_, err = s.prepareAndSubmit(createParticipantTx,
		[]sdk.Address{node.account.accountAddress, s.blockchain.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{node.account.accountSigner, s.blockchain.ServiceKey().Signer(), s.dkgSigner},
	)
	require.NoError(s.T(), err)

	// verify that nodeID was registered
	result := s.executeScript(templates.GenerateGetDKGNodeIsRegisteredScript(s.env),
		[][]byte{
			jsoncdc.MustEncode(
				cadence.String(node.account.accountID)),
		})
	assert.Equal(s.T(), cadence.NewBool(true), result)
}

// sendDummyTx submits a transaction from the service account
func (s *DKGSuite) sendDummyTx() (*flow.Block, error) {
	// we are using an account-creation transaction but it doesnt matter; we
	// could be using anything other transaction
	createAccountTx := sdktemplates.CreateAccount(
		[]*sdk.AccountKey{test.AccountKeyGenerator().New()},
		[]sdktemplates.Contract{},
		s.blockchain.ServiceKey().Address).
		SetProposalKey(
			s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address)

	block, err := s.prepareAndSubmit(createAccountTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer()},
	)
	return block, err
}

func (s *DKGSuite) isDKGCompleted() bool {
	template := templates.GenerateGetDKGCompletedScript(s.env)
	value := s.executeScript(template, nil)
	return value.ToGoValue().(bool)
}

func (s *DKGSuite) getResult() []string {
	script := fmt.Sprintf(`
	import FlowDKG from 0x%s

	pub fun main(): [String?]? {
		return FlowDKG.dkgCompleted()
	} `,
		s.env.DkgAddress,
	)

	res := s.executeScript([]byte(script), nil)
	value := res.(cadence.Optional).ToGoValue()
	if value == nil {
		return []string{}
	}
	dkgResult := []string{}
	for _, item := range value.([]interface{}) {
		s := item.(string)
		dkgResult = append(dkgResult, s)
	}

	return dkgResult
}

func (s *DKGSuite) initEngines(node *node, ids flow.IdentityList) {
	core := testutil.GenericNodeFromParticipants(s.T(), s.hub, node.account.netID, ids, s.chainID)
	core.Log = zerolog.New(os.Stdout).Level(zerolog.DebugLevel)

	// the viewsObserver is used by the reactor engine to subscribe to new views
	// being finalized
	viewsObserver := gadgets.NewViews()
	core.ProtocolEvents.AddConsumer(viewsObserver)

	// keyKeys is used to store the private key resulting from the node's
	// participation in the DKG run
	dkgKeys, err := badger.NewDKGKeys(core.Metrics, core.SecretsDB)
	s.Require().NoError(err)

	// brokerTunnel is used to communicate between the messaging engine and the
	// DKG broker/controller
	brokerTunnel := dkg.NewBrokerTunnel()

	// messagingEngine is a network engine that is used by nodes to exchange
	// private DKG messages
	messagingEngine, err := dkgeng.NewMessagingEngine(
		core.Log,
		core.Net,
		core.Me,
		brokerTunnel,
	)
	require.NoError(s.T(), err)

	controllerFactoryLogger := zerolog.New(os.Stdout)
	if s.checkDKGUnhappy {
		// We add a hook to the logger such that the test fails if the broker writes
		// a Warn log, which happens when it flags or disqualifies a node
		hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			if level == zerolog.WarnLevel {
				s.T().Fatal("DKG flagging misbehaviour")
			}
		})
		controllerFactoryLogger = zerolog.New(os.Stdout).Hook(hook)
	}

	// create a config with no delays for tests
	config := dkg.ControllerConfig{
		BaseStartDelay:                0,
		BaseHandleFirstBroadcastDelay: 0,
	}

	// the reactor engine reacts to new views being finalized and drives the
	// DKG protocol
	reactorEngine := dkgeng.NewReactorEngine(
		core.Log,
		core.Me,
		core.State,
		dkgKeys,
		dkg.NewControllerFactory(
			controllerFactoryLogger,
			core.Me,
			[]module.DKGContractClient{node.dkgContractClient},
			brokerTunnel,
			config,
		),
		viewsObserver,
	)

	// reactorEngine consumes the EpochSetupPhaseStarted event
	core.ProtocolEvents.AddConsumer(reactorEngine)

	node.GenericNode = core
	node.keyStorage = dkgKeys
	node.messagingEngine = messagingEngine
	node.reactorEngine = reactorEngine
}

// prepareAndSubmit adds a block reference and signs a transaction before
// submitting it via the admin emulator client.
func (s *DKGSuite) prepareAndSubmit(tx *sdk.Transaction, signerAddresses []sdk.Address, signers []sdkcrypto.Signer) (*flow.Block, error) {

	// set block reference
	latestBlock, err := s.adminEmulatorClient.GetLatestBlock(nil, true)
	if err != nil {
		return nil, fmt.Errorf("could not get latest block: %w", err)
	}
	tx.SetReferenceBlockID(latestBlock.ID)

	// sign transaction with each signer
	for i := len(signerAddresses) - 1; i >= 0; i-- {
		signerAddress := signerAddresses[i]
		signer := signers[i]

		if i == 0 {
			err := tx.SignEnvelope(signerAddress, 0, signer)
			require.NoError(s.T(), err)
		} else {
			err := tx.SignPayload(signerAddress, 0, signer)
			require.NoError(s.T(), err)

		}
	}

	// submit transaction
	return s.adminEmulatorClient.Submit(tx)
}

// executeScript runs a cadence script on the emulator blockchain
func (s *DKGSuite) executeScript(script []byte, arguments [][]byte) cadence.Value {
	result, err := s.blockchain.ExecuteScript(script, arguments)
	require.NoError(s.T(), err)
	require.True(s.T(), result.Succeeded())
	return result.Value
}
