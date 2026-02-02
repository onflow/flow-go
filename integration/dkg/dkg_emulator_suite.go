package dkg

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/onflow/crypto"
	"golang.org/x/exp/slices"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"

	dkgeng "github.com/onflow/flow-go/engine/consensus/dkg"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	emulator "github.com/onflow/flow-go/integration/internal/emulator"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

const numberOfNodes = 10

// EmulatorSuite tests the DKG protocol against the DKG smart contract running on the Emulator.
type EmulatorSuite struct {
	suite.Suite

	chainID                flow.ChainID
	hub                    *stub.Hub // in-mem test network
	env                    templates.Environment
	blockchain             emulator.Emulator
	adminEmulatorClient    *utils.EmulatorClient
	adminDKGContractClient *dkg.Client
	dkgAddress             sdk.Address
	dkgAccountKey          *sdk.AccountKey
	dkgSigner              sdkcrypto.Signer
	checkDKGUnhappy        bool // activate log hook for DKGBroker to check if the DKG core is flagging misbehaviours
	serviceAccountAddress  sdk.Address
	netIDs                 flow.IdentityList
	nodeAccounts           []*nodeAccount
	nodes                  []*node
}

func (s *EmulatorSuite) SetupTest() {
	s.initEmulator()
	s.deployDKGContract()
	s.setupDKGAdmin()

	boostrapNodesInfo := unittest.PrivateNodeInfosFixture(numberOfNodes, unittest.WithRole(flow.RoleConsensus))
	slices.SortFunc(boostrapNodesInfo, func(lhs, rhs bootstrap.NodeInfo) int {
		return flow.IdentifierCanonical(lhs.NodeID, rhs.NodeID)
	})
	for _, id := range boostrapNodesInfo {
		s.nodeAccounts = append(s.nodeAccounts, s.createAndFundAccount(id))
		s.netIDs = append(s.netIDs, id.Identity())
	}

	for _, acc := range s.nodeAccounts {
		s.nodes = append(s.nodes, s.createNode(acc))
	}

	s.startDKGWithParticipants(s.nodeAccounts)

	for _, node := range s.nodes {
		s.claimDKGParticipant(node)
	}
}

func (s *EmulatorSuite) BeforeTest(_, testName string) {
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

func (s *EmulatorSuite) TearDownTest() {
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
func (s *EmulatorSuite) initEmulator() {
	s.chainID = flow.Emulator

	blockchain, err := emulator.New(
		emulator.WithTransactionExpiry(flow.DefaultTransactionExpiry),
		emulator.WithStorageLimitEnabled(false),
	)
	s.Require().NoError(err)

	s.blockchain = blockchain
	s.serviceAccountAddress = sdk.Address(s.blockchain.ServiceKey().Address)
	s.adminEmulatorClient = utils.NewEmulatorClient(blockchain)

	s.hub = stub.NewNetworkHub()
}

// deployDKGContract deploys the DKG contract to the emulator and initializes
// the admin DKG contract client
func (s *EmulatorSuite) deployDKGContract() {
	// create new account keys for the DKG contract
	dkgAccountKey, dkgAccountSigner := test.AccountKeyGenerator().NewWithSigner()

	// deploy the contract to the emulator
	dkgAddress, err := s.adminEmulatorClient.CreateAccount([]*sdk.AccountKey{dkgAccountKey}, []sdktemplates.Contract{
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
		flow.ZeroID,
		s.dkgSigner,
		s.dkgAddress.String(),
		s.dkgAddress.String(), 0)
}

func (s *EmulatorSuite) setupDKGAdmin() {
	setUpAdminTx := sdk.NewTransaction().
		SetScript(templates.GeneratePublishDKGAdminScript(s.env)).
		SetComputeLimit(9999).
		SetProposalKey(
			s.serviceAccountAddress,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.serviceAccountAddress).
		AddAuthorizer(s.dkgAddress)
	signer, err := s.blockchain.ServiceKey().Signer()
	require.NoError(s.T(), err)
	_, err = s.prepareAndSubmit(setUpAdminTx,
		[]sdk.Address{s.serviceAccountAddress, s.dkgAddress},
		[]sdkcrypto.Signer{signer, s.dkgSigner},
	)
	require.NoError(s.T(), err)
}

// createAndFundAccount creates a nodeAccount and funds it in the emulator
func (s *EmulatorSuite) createAndFundAccount(netID bootstrap.NodeInfo) *nodeAccount {
	accountPrivateKey := lib.RandomPrivateKey()
	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetSigAlgo(sdkcrypto.ECDSA_P256).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)
	accountID := netID.NodeID.String()
	accountSigner, err := sdkcrypto.NewInMemorySigner(accountPrivateKey, accountKey.HashAlgo)
	require.NoError(s.T(), err)

	sc := systemcontracts.SystemContractsForChain(s.chainID)

	/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	create Flow account
	~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

	newAccountAddress, err := s.adminEmulatorClient.CreateAccount(
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
				  let sentVault: @{FungibleToken.Vault}
				  prepare(signer: auth(BorrowValue) &Account) {
					let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
					  ?? panic("failed to borrow reference to sender vault")
					self.sentVault <- vaultRef.withdraw(amount: amount)
				  }
				  execute {
					let receiverRef =  getAccount(recipient)
					  .capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
						?? panic("failed to borrow reference to recipient vault")
					receiverRef.deposit(from: <-self.sentVault)
				  }
				}`,
					sc.FungibleToken.Address.Hex(),
					sc.FlowToken.Address.Hex(),
				))).
		AddAuthorizer(s.serviceAccountAddress).
		SetProposalKey(
			s.serviceAccountAddress,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber,
		).
		SetPayer(s.serviceAccountAddress)

	err = fundAccountTx.AddArgument(cadence.UFix64(1_000_000))
	require.NoError(s.T(), err)
	err = fundAccountTx.AddArgument(cadence.NewAddress(newAccountAddress))
	require.NoError(s.T(), err)
	signer, err := s.blockchain.ServiceKey().Signer()
	require.NoError(s.T(), err)
	_, err = s.prepareAndSubmit(fundAccountTx,
		[]sdk.Address{s.serviceAccountAddress},
		[]sdkcrypto.Signer{signer},
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
func (s *EmulatorSuite) createNode(account *nodeAccount) *node {
	emulatorClient := utils.NewEmulatorClient(s.blockchain)
	contractClient := dkg.NewClient(
		zerolog.Nop(),
		emulatorClient,
		flow.ZeroID,
		account.accountSigner,
		s.dkgAddress.String(),
		account.accountAddress.String(),
		0,
	)
	dkgClientWrapper := NewDKGClientWrapper(contractClient)
	return &node{
		t:                 s.T(),
		account:           account,
		dkgContractClient: dkgClientWrapper,
	}
}

func (s *EmulatorSuite) startDKGWithParticipants(accounts []*nodeAccount) {
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
		SetComputeLimit(9999).
		SetProposalKey(
			s.serviceAccountAddress,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.serviceAccountAddress).
		AddAuthorizer(s.dkgAddress)

	err := startDKGTx.AddArgument(cadence.NewArray(valueNodeIDs))
	require.NoError(s.T(), err)
	signer, err := s.blockchain.ServiceKey().Signer()
	require.NoError(s.T(), err)
	_, err = s.prepareAndSubmit(startDKGTx,
		[]sdk.Address{s.serviceAccountAddress, s.dkgAddress},
		[]sdkcrypto.Signer{signer, s.dkgSigner},
	)
	require.NoError(s.T(), err)

	// sanity check: verify that DKG was started with correct node IDs
	result := s.executeScript(templates.GenerateGetConsensusNodesScript(s.env), nil)
	require.IsType(s.T(), cadence.Array{}, result)
	assert.ElementsMatch(s.T(), valueNodeIDs, result.(cadence.Array).Values)
}

func (s *EmulatorSuite) claimDKGParticipant(node *node) {
	createParticipantTx := sdk.NewTransaction().
		SetScript(templates.GenerateCreateDKGParticipantScript(s.env)).
		SetComputeLimit(9999).
		SetProposalKey(
			s.serviceAccountAddress,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber,
		).
		SetPayer(node.account.accountAddress).
		AddAuthorizer(node.account.accountAddress)

	err := createParticipantTx.AddArgument(cadence.NewAddress(s.dkgAddress))
	require.NoError(s.T(), err)
	nodeID, err := cadence.NewString(node.account.accountID)
	require.NoError(s.T(), err)
	err = createParticipantTx.AddArgument(nodeID)
	require.NoError(s.T(), err)
	signer, err := s.blockchain.ServiceKey().Signer()
	require.NoError(s.T(), err)
	_, err = s.prepareAndSubmit(createParticipantTx,
		[]sdk.Address{node.account.accountAddress, s.serviceAccountAddress},
		[]sdkcrypto.Signer{node.account.accountSigner, signer},
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
func (s *EmulatorSuite) sendDummyTx() (*flow.Block, error) {
	// we are using an account-creation transaction but it doesnt matter; we
	// could be using anything other transaction
	createAccountTx, err := sdktemplates.CreateAccount(
		[]*sdk.AccountKey{test.AccountKeyGenerator().New()},
		[]sdktemplates.Contract{},
		s.serviceAccountAddress)
	if err != nil {
		return nil, err
	}
	createAccountTx.
		SetProposalKey(
			s.serviceAccountAddress,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.serviceAccountAddress)

	signer, err := s.blockchain.ServiceKey().Signer()
	require.NoError(s.T(), err)
	block, err := s.prepareAndSubmit(createAccountTx,
		[]sdk.Address{s.serviceAccountAddress},
		[]sdkcrypto.Signer{signer},
	)
	return block, err
}

func (s *EmulatorSuite) isDKGCompleted() bool {
	template := templates.GenerateGetDKGCompletedScript(s.env)
	value := s.executeScript(template, nil)
	return bool(value.(cadence.Bool))
}

// getParametersAndResult retrieves the DKG setup parameters (`flow.DKGIndexMap`) and the DKG result from the DKG white-board smart contract.
func (s *EmulatorSuite) getParametersAndResult() (flow.DKGIndexMap, crypto.PublicKey, []crypto.PublicKey) {
	res := s.executeScript(templates.GenerateGetDKGCanonicalFinalSubmissionScript(s.env), nil)
	value := res.(cadence.Optional).Value
	if value == nil {
		s.Fail("DKG result is nil")
	}

	decodePubkey := func(r string) crypto.PublicKey {
		pkBytes, err := hex.DecodeString(r)
		require.NoError(s.T(), err)
		pk, err := crypto.DecodePublicKey(crypto.BLSBLS12381, pkBytes)
		require.NoError(s.T(), err)
		return pk
	}

	fields := value.(cadence.Struct).FieldsMappedByName()
	groupKey := decodePubkey(string(UnwrapOptional[cadence.String](fields["groupPubKey"])))

	dkgKeyShares := CadenceArrayTo(UnwrapOptional[cadence.Array](fields["pubKeys"]), func(value cadence.Value) crypto.PublicKey {
		return decodePubkey(string(value.(cadence.String)))
	})

	cdcIndexMap := CDCToDKGIDMapping(UnwrapOptional[cadence.Dictionary](fields["idMapping"]))
	indexMap := make(flow.DKGIndexMap, len(cdcIndexMap))
	for k, v := range cdcIndexMap {
		nodeID, err := flow.HexStringToIdentifier(k)
		require.NoError(s.T(), err)
		indexMap[nodeID] = v
	}

	return indexMap, groupKey, dkgKeyShares
}

func (s *EmulatorSuite) initEngines(node *node, ids flow.IdentityList) {
	core := testutil.GenericNodeFromParticipants(s.T(), s.hub, node.account.netID, ids, s.chainID)
	core.Log = zerolog.New(os.Stdout).Level(zerolog.DebugLevel)

	// the viewsObserver is used by the reactor engine to subscribe to new views
	// being finalized
	viewsObserver := gadgets.NewViews()
	core.ProtocolEvents.AddConsumer(viewsObserver)

	// dkgState is used to store the private key resulting from the node's
	// participation in the DKG run
	dkgState, err := badger.NewRecoverableRandomBeaconStateMachine(core.Metrics, core.SecretsDB, core.Me.NodeID())
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
		metrics.NewNoopCollector(),
		dkgeng.DefaultMessagingEngineConfig(),
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

	// the reactor engine reacts to new views being finalized and drives the
	// DKG protocol
	reactorEngine := dkgeng.NewReactorEngine(
		core.Log,
		core.Me,
		core.State,
		dkgState,
		dkg.NewControllerFactory(
			controllerFactoryLogger,
			core.Me,
			[]module.DKGContractClient{node.dkgContractClient},
			brokerTunnel,
		),
		viewsObserver,
	)

	// reactorEngine consumes the EpochSetupPhaseStarted event
	core.ProtocolEvents.AddConsumer(reactorEngine)

	node.GenericNode = core
	node.messagingEngine = messagingEngine
	node.dkgState = dkgState
	node.reactorEngine = reactorEngine
}

// prepareAndSubmit adds a block reference and signs a transaction before
// submitting it via the admin emulator client.
func (s *EmulatorSuite) prepareAndSubmit(tx *sdk.Transaction, signerAddresses []sdk.Address, signers []sdkcrypto.Signer) (*flow.Block, error) {

	// set block reference
	latestBlock, err := s.adminEmulatorClient.GetLatestBlock(context.Background(), true)
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
func (s *EmulatorSuite) executeScript(script []byte, arguments [][]byte) cadence.Value {
	result, err := s.blockchain.ExecuteScript(script, arguments)
	require.NoError(s.T(), err)
	require.True(s.T(), result.Succeeded())
	return result.Value
}

func UnwrapOptional[T cadence.Value](optional cadence.Value) T {
	return optional.(cadence.Optional).Value.(T)
}

func CadenceArrayTo[T any](arr cadence.Value, convert func(cadence.Value) T) []T {
	out := make([]T, len(arr.(cadence.Array).Values))
	for i := range out {
		out[i] = convert(arr.(cadence.Array).Values[i])
	}
	return out
}

func CDCToDKGIDMapping(cdc cadence.Value) map[string]int {
	idMappingCDC := cdc.(cadence.Dictionary)
	idMapping := make(map[string]int, len(idMappingCDC.Pairs))
	for _, pair := range idMappingCDC.Pairs {
		nodeID := string(pair.Key.(cadence.String))
		index := pair.Value.(cadence.Int).Int()
		idMapping[nodeID] = index
	}
	return idMapping
}
