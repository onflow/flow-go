package dkg

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	emulator "github.com/onflow/flow-emulator"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	emulatormod "github.com/onflow/flow-go/module/emulator"
	"github.com/onflow/flow-go/utils/unittest"
)

type ClientSuite struct {
	suite.Suite

	contractClient *Client

	env            templates.Environment
	blockchain     *emulator.Blockchain
	emulatorClient *emulatormod.EmulatorClient

	dkgAddress    sdk.Address
	dkgAccountKey *sdk.AccountKey
	dkgSigner     sdkcrypto.Signer
}

func TestDKGClient(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

// Setup Test creates the blockchain client, the emulated blockchain and deploys
// the DKG contract to the emulator
func (s *ClientSuite) SetupTest() {
	blockchain, err := emulator.NewBlockchain(emulator.WithStorageLimitEnabled(false))
	require.NoError(s.T(), err)

	s.blockchain = blockchain
	s.emulatorClient = emulatormod.NewEmulatorClient(blockchain)

	// deploy contract
	s.deployDKGContract()

	s.contractClient = NewClient(zerolog.Nop(), s.emulatorClient, s.dkgSigner, s.dkgAddress.String(), s.dkgAddress.String(), 0)
}

func (s *ClientSuite) deployDKGContract() {

	// create new account keys for the DKG contract
	accountKey, signer := test.AccountKeyGenerator().NewWithSigner()
	code := contracts.FlowDKG()

	// deploy the contract to the emulator
	dkgAddress, err := s.blockchain.CreateAccount([]*sdk.AccountKey{accountKey}, []sdktemplates.Contract{
		{
			Name:   "FlowDKG",
			Source: string(code),
		},
	})
	require.NoError(s.T(), err)

	env := templates.Environment{
		DkgAddress: dkgAddress.Hex(),
	}

	s.env = env
	s.dkgAddress = dkgAddress
	s.dkgAccountKey = accountKey
	s.dkgSigner = signer
}

// TestBroadcast broadcasts and messages and verifies that no errors are thrown
// Note: Contract functionality tested by `flow-core-contracts`
func (s *ClientSuite) TestBroadcast() {

	// create single dkg participant
	participants := unittest.IdentifierListFixture(1)

	// set up DKG with Participants
	clients := s.prepareDKG(participants)

	// create DKG message fixture
	msg := unittest.DKGBroadcastMessageFixture()

	// broadcast messsage a random broadcast message and verify that there were no errors
	err := clients[0].Broadcast(*msg)
	assert.NoError(s.T(), err)
}

// TestDKGContractClient submits a single broadcast to the DKG contract, reads the broadcast
// to verify what we broadcasted was what was received
func (s *ClientSuite) TestBroadcastReadSingle() {

	// create single dkg participant
	participants := unittest.IdentifierListFixture(1)

	// set up DKG with Participants
	clients := s.prepareDKG(participants)

	// create DKG message fixture
	msg := unittest.DKGBroadcastMessageFixture()

	// broadcast messsage a random broadcast message and verify that there were no errors
	err := clients[0].Broadcast(*msg)
	assert.NoError(s.T(), err)

	// read latest broadcast messages
	block, err := s.blockchain.GetLatestBlock()
	require.NoError(s.T(), err)

	// verify the data recieved with data sent
	messages, err := clients[0].ReadBroadcast(0, block.ID())
	require.NoError(s.T(), err)
	assert.Len(s.T(), messages, 1)

	broadcastedMsg := messages[0]
	assert.Equal(s.T(), msg.DKGInstanceID, broadcastedMsg.DKGInstanceID)
	assert.Equal(s.T(), msg.Data, broadcastedMsg.Data)
	assert.Equal(s.T(), msg.Orig, broadcastedMsg.Orig)
	assert.Equal(s.T(), msg.Signature, broadcastedMsg.Signature)
}

// TestNilDKGSubmission tests that even with `nil` DKG public keys the `SubmitResult`
// still proceeds with no errors
func (s *ClientSuite) TestNilDKGSubmission() {

	// create two participants
	participants := unittest.IdentifierListFixture(2)

	// prepare DKG
	clients := s.prepareDKG(participants)

	// generate list of public keys
	numberOfNodes := len(participants)
	publicKeys := make([]crypto.PublicKey, 0, numberOfNodes+1)
	for i := 0; i < numberOfNodes; i++ {
		publicKeys = append(publicKeys, nil)
	}

	// create a nil group public key
	var groupPublicKey crypto.PublicKey

	// submit empty nil keys for each participant
	for _, client := range clients {
		err := client.SubmitResult(groupPublicKey, publicKeys)
		require.NoError(s.T(), err)
	}
}

// TestSubmitResult creates random DKG public key submission and verifys that transaction was
// submitted with no errors
func (s *ClientSuite) TestSubmitResult() {
	// create single dkg participant
	participants := unittest.IdentifierListFixture(1)

	// set up DKG with Participants
	clients := s.prepareDKG(participants)

	// generate list of public keys
	numberOfNodes := len(participants)
	publicKeys := make([]crypto.PublicKey, 0, numberOfNodes)
	for i := 0; i < numberOfNodes; i++ {
		privateKey := unittest.KeyFixture(crypto.BLSBLS12381)
		publicKeys = append(publicKeys, privateKey.PublicKey())
	}
	// create a group public key
	groupPublicKey := unittest.KeyFixture(crypto.BLSBLS12381).PublicKey()

	err := clients[0].SubmitResult(groupPublicKey, publicKeys)
	require.NoError(s.T(), err)
}

func (s *ClientSuite) prepareDKG(participants []flow.Identifier) []*Client {

	// set up the admin account
	s.setUpAdmin()

	nodeIDs := make([]flow.Identifier, len(participants))
	accountKeys := make([]*sdk.AccountKey, len(participants))
	signers := make([]sdkcrypto.Signer, len(participants))
	addresses := make([]sdk.Address, len(participants))

	for index, participant := range participants {

		nodeIDs[index] = participant

		// create account key, address and signer for participant
		accountKey, signer := test.AccountKeyGenerator().NewWithSigner()
		address, err := s.blockchain.CreateAccount([]*sdk.AccountKey{accountKey}, nil)
		require.NoError(s.T(), err)

		accountKeys[index], addresses[index], signers[index] = accountKey, address, signer
	}

	// start DKG with participants
	s.startDKGWithParticipants(nodeIDs)

	for index := range participants {
		// create participant resource
		s.createParticipant(nodeIDs[index], addresses[index], signers[index])
	}

	// create clients for each participant
	clients := make([]*Client, len(participants))
	for index := range participants {
		clients[index] = NewClient(zerolog.Nop(), s.emulatorClient, signers[index], s.dkgAddress.String(), addresses[index].String(), 0)
	}

	return clients
}

func (s *ClientSuite) setUpAdmin() {

	// set up admin resource
	setUpAdminTx := sdk.NewTransaction().
		SetScript(templates.GeneratePublishDKGParticipantScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.blockchain.ServiceKey().Address, s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.dkgAddress)
	s.signAndSubmit(setUpAdminTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.dkgSigner},
	)
}

func (s *ClientSuite) startDKGWithParticipants(nodeIDs []flow.Identifier) {

	// convert node identifiers to candece.Value to be passed in as TX argument
	valueNodeIDs := make([]cadence.Value, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		cdcNodeID, err := cadence.NewString(nodeID.String())
		if err != nil {
			panic(err)
		}
		valueNodeIDs = append(valueNodeIDs, cdcNodeID)
	}

	// start DKG using admin resource
	startDKGTx := sdk.NewTransaction().
		SetScript(templates.GenerateStartDKGScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.blockchain.ServiceKey().Address, s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.dkgAddress)

	err := startDKGTx.AddArgument(cadence.NewArray(valueNodeIDs))
	require.NoError(s.T(), err)

	s.signAndSubmit(startDKGTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.dkgSigner},
	)

	// sanity check: verify that DKG was started with correct node IDs
	result := s.executeScript(templates.GenerateGetConsensusNodesScript(s.env), nil)
	assert.Equal(s.T(), cadence.NewArray(valueNodeIDs), result)
}

func (s *ClientSuite) createParticipant(nodeID flow.Identifier, authoriser sdk.Address, signer sdkcrypto.Signer) {

	// create DKG partcipant
	createParticipantTx := sdk.NewTransaction().
		SetScript(templates.GenerateCreateDKGParticipantScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.blockchain.ServiceKey().Address, s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(authoriser)

	err := createParticipantTx.AddArgument(cadence.NewAddress(s.dkgAddress))
	require.NoError(s.T(), err)

	cdcNodeID, err := cadence.NewString(nodeID.String())
	if err != nil {
		panic(err)
	}
	err = createParticipantTx.AddArgument(cdcNodeID)
	require.NoError(s.T(), err)

	s.signAndSubmit(createParticipantTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, authoriser},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), signer},
	)

	// verify that nodeID was registered
	result := s.executeScript(templates.GenerateGetDKGNodeIsRegisteredScript(s.env),
		[][]byte{jsoncdc.MustEncode(cadence.String(nodeID.String()))})
	assert.Equal(s.T(), cadence.NewBool(true), result)

}

func (s *ClientSuite) signAndSubmit(tx *sdk.Transaction, signerAddresses []sdk.Address, signers []sdkcrypto.Signer) {

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
	_, err := s.emulatorClient.Submit(tx)
	require.NoError(s.T(), err)
}

func (s *ClientSuite) executeScript(script []byte, arguments [][]byte) cadence.Value {

	// execute script
	result, err := s.blockchain.ExecuteScript(script, arguments)
	require.NoError(s.T(), err)
	require.True(s.T(), result.Succeeded())

	return result.Value
}
