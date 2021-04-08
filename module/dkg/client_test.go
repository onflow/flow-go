package dkg

import (
	"testing"

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
	"github.com/onflow/flow-go/utils/unittest"
)

type ClientSuite struct {
	suite.Suite

	client *Client

	env            templates.Environment
	emulator       *emulator.Blockchain
	emulatorClient *EmulatorClient

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
	emulator, err := emulator.NewBlockchain()
	require.NoError(s.T(), err)

	s.emulator = emulator
	s.emulatorClient = NewEmulatorClient(emulator)

	// deploy contract
	s.deployDKGContract()

	// Note: using DKG address as DKG participant to avoid funding a new account key
	s.client = NewClient(s.emulatorClient, s.dkgSigner, s.dkgAddress.String(), s.dkgAddress.String(), 0)

	// set up admin resource
	s.setUpAdmin()
}

// TestBroadcast broadcasts and messages and verifies that no errors are thrown
// Note: Contract functionality tested by `flow-core-contracts`
func (s *ClientSuite) TestBroadcast() {

	// dkg node ids and paricipant node id
	nodeID := unittest.IdentifierFixture()
	dkgNodeIDStrings := make([]cadence.Value, 1)
	dkgNodeIDStrings[0] = cadence.NewString(nodeID.String())

	// create DKG message fixture
	msg := unittest.DKGMessageFixture()

	// start dkf with 1 participant
	s.startDKGWithParticipants(dkgNodeIDStrings)

	// create participant resource
	s.createParticipant(nodeID.String())

	// broadcast messsage a random broadcast message and verify that there were no errors
	err := s.client.Broadcast(*msg)
	assert.NoError(s.T(), err)
}

// TestDKGContractClient submits a single broadcast to the DKG contract, reads the broadcast
// to verify what we broadcasted was what was received
func (s *ClientSuite) TestBroadcastReadSingle() {

	// dkg partcipant node ID and participants
	nodeID := unittest.IdentifierFixture()
	dkgNodeIDStrings := make([]cadence.Value, 1)
	dkgNodeIDStrings[0] = cadence.NewString(nodeID.String())

	// start dkf with 1 participant
	s.startDKGWithParticipants(dkgNodeIDStrings)

	// create participant resource
	s.createParticipant(nodeID.String())

	// create DKG message fixture
	msg := unittest.DKGMessageFixture()

	// broadcast messsage a random broadcast message and verify that there were no errors
	err := s.client.Broadcast(*msg)
	assert.NoError(s.T(), err)

	// read latest broadcast messages
	block, err := s.emulator.GetLatestBlock()
	require.NoError(s.T(), err)

	// verify the data recieved with data sent
	messages, err := s.client.ReadBroadcast(0, block.ID())
	require.NoError(s.T(), err)
	assert.Len(s.T(), messages, 1)

	broadcastedMsg := messages[0]
	assert.Equal(s.T(), msg.DKGInstanceID, broadcastedMsg.DKGInstanceID)
	assert.Equal(s.T(), msg.Data, broadcastedMsg.Data)
	assert.Equal(s.T(), msg.Orig, broadcastedMsg.Orig)
}

func (s *ClientSuite) TestSubmitResult() {
	nodeID := unittest.IdentifierFixture()
	dkgNodeIDStrings := make([]cadence.Value, 1)
	dkgNodeIDStrings[0] = cadence.NewString(nodeID.String())

	// start dkf with 1 participant
	s.startDKGWithParticipants(dkgNodeIDStrings)

	// create participant resource
	s.createParticipant(nodeID.String())

	// create DKG message fixture
	msg := unittest.DKGMessageFixture()

	// broadcast messsage a random broadcast message and verify that there were no errors
	err := s.client.Broadcast(*msg)
	assert.NoError(s.T(), err)

	numberOfKeys := 1

	// generate list of public keys
	publicKeys := make([]crypto.PublicKey, 0, numberOfKeys)
	for i := 0; i < numberOfKeys; i++ {
		privateKey := unittest.KeyFixture(crypto.BLSBLS12381)
		publicKeys = append(publicKeys, privateKey.PublicKey())
	}

	// create a group public key
	groupPublicKey := unittest.KeyFixture(crypto.BLSBLS12381).PublicKey()

	err = s.client.SubmitResult(groupPublicKey, publicKeys)
	require.NoError(s.T(), err)
}

func (s *ClientSuite) deployDKGContract() {

	// create new account keys for the DKG contract
	accountKey, signer := test.AccountKeyGenerator().NewWithSigner()
	code := contracts.FlowDKG()

	// deploy the contract to the emulator
	dkgAddress, err := s.emulator.CreateAccount([]*sdk.AccountKey{accountKey}, []sdktemplates.Contract{
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

func (s *ClientSuite) setUpAdmin() {

	// set up admin resource
	tx := sdk.NewTransaction().
		SetScript(templates.GeneratePublishDKGParticipantScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.emulator.ServiceKey().Address, s.emulator.ServiceKey().Index,
			s.emulator.ServiceKey().SequenceNumber).
		SetPayer(s.emulator.ServiceKey().Address).
		AddAuthorizer(s.dkgAddress)

	s.signAndSubmit(tx,
		[]sdk.Address{s.emulator.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.emulator.ServiceKey().Signer(), s.dkgSigner},
	)
}

func (s *ClientSuite) startDKGWithParticipants(nodeIDs []cadence.Value) {

	// start DKG using admin resource
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateStartDKGScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.emulator.ServiceKey().Address, s.emulator.ServiceKey().Index,
			s.emulator.ServiceKey().SequenceNumber).
		SetPayer(s.emulator.ServiceKey().Address).
		AddAuthorizer(s.dkgAddress)

	err := tx.AddArgument(cadence.NewArray(nodeIDs))
	require.NoError(s.T(), err)

	s.signAndSubmit(tx,
		[]sdk.Address{s.emulator.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.emulator.ServiceKey().Signer(), s.dkgSigner},
	)

	// sanity check: verify that DKG was started with correct node IDs
	result := s.executeScript(templates.GenerateGetConsensusNodesScript(s.env), nil)
	assert.Equal(s.T(), cadence.NewArray(nodeIDs), result)
}

func (s *ClientSuite) createParticipant(nodeID string) {

	// create DKG partcipant
	createParticipantTx := sdk.NewTransaction().
		SetScript(templates.GenerateCreateDKGParticipantScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.emulator.ServiceKey().Address, s.emulator.ServiceKey().Index,
			s.emulator.ServiceKey().SequenceNumber).
		SetPayer(s.emulator.ServiceKey().Address).
		AddAuthorizer(s.dkgAddress)

	err := createParticipantTx.AddArgument(cadence.NewAddress(s.dkgAddress))
	require.NoError(s.T(), err)
	err = createParticipantTx.AddArgument(cadence.NewString(nodeID))
	require.NoError(s.T(), err)

	s.signAndSubmit(createParticipantTx,
		[]sdk.Address{s.emulator.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.emulator.ServiceKey().Signer(), s.dkgSigner},
	)

	// verify that nodeID was registered
	result := s.executeScript(templates.GenerateGetDKGNodeIsRegisteredScript(s.env),
		[][]byte{jsoncdc.MustEncode(cadence.String(nodeID))})
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
	err := s.emulatorClient.Submit(tx)
	require.NoError(s.T(), err)
}

func (s *ClientSuite) executeScript(script []byte, arguments [][]byte) cadence.Value {

	// execute script
	result, err := s.emulator.ExecuteScript(script, arguments)
	require.NoError(s.T(), err)
	require.True(s.T(), result.Succeeded())

	return result.Value
}
