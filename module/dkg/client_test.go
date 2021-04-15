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
	blockchain, err := emulator.NewBlockchain()
	require.NoError(s.T(), err)

	s.blockchain = blockchain
	s.emulatorClient = emulatormod.NewEmulatorClient(blockchain)

	// deploy contract
	s.deployDKGContract()

	// Note: using DKG address as DKG participant to avoid funding a new account key
	s.contractClient, err = NewClient(zerolog.Nop(), s.emulatorClient, s.dkgSigner, s.dkgAddress.String(), s.dkgAddress.String(), 0)
	require.NoError(s.T(), err)

	s.setUpAdmin()
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

	// dkg node ids and paricipant node id
	nodeID := unittest.IdentifierFixture()
	dkgNodeIDStrings := make([]flow.Identifier, 1)
	dkgNodeIDStrings[0] = nodeID

	// start dkf with 1 participant
	s.startDKGWithParticipants(dkgNodeIDStrings)

	// create participant resource
	s.createParticipant(nodeID)

	// create DKG message fixture
	msg := unittest.DKGBroadcastMessageFixture()

	// broadcast messsage a random broadcast message and verify that there were no errors
	err := s.contractClient.Broadcast(*msg)
	assert.NoError(s.T(), err)
}

// TestDKGContractClient submits a single broadcast to the DKG contract, reads the broadcast
// to verify what we broadcasted was what was received
func (s *ClientSuite) TestBroadcastReadSingle() {

	// dkg partcipant node ID and participants
	nodeID := unittest.IdentifierFixture()
	dkgNodeIDStrings := make([]flow.Identifier, 1)
	dkgNodeIDStrings[0] = nodeID

	// start dkf with 1 participant
	s.startDKGWithParticipants(dkgNodeIDStrings)

	// create participant resource
	s.createParticipant(nodeID)

	// create DKG message fixture
	msg := unittest.DKGBroadcastMessageFixture()

	// broadcast messsage a random broadcast message and verify that there were no errors
	err := s.contractClient.Broadcast(*msg)
	assert.NoError(s.T(), err)

	// read latest broadcast messages
	block, err := s.blockchain.GetLatestBlock()
	require.NoError(s.T(), err)

	// verify the data recieved with data sent
	messages, err := s.contractClient.ReadBroadcast(0, block.ID())
	require.NoError(s.T(), err)
	assert.Len(s.T(), messages, 1)

	broadcastedMsg := messages[0]
	assert.Equal(s.T(), msg.DKGInstanceID, broadcastedMsg.DKGInstanceID)
	assert.Equal(s.T(), msg.Data, broadcastedMsg.Data)
	assert.Equal(s.T(), msg.Orig, broadcastedMsg.Orig)
	assert.Equal(s.T(), msg.Signature, broadcastedMsg.Signature)
}

func (s *ClientSuite) TestSubmitResult() {
	nodeID := unittest.IdentifierFixture()
	dkgNodeIDStrings := make([]flow.Identifier, 1)
	dkgNodeIDStrings[0] = nodeID

	// start dkf with 1 participant
	s.startDKGWithParticipants(dkgNodeIDStrings)

	// create participant resource
	s.createParticipant(nodeID)

	// generate list of public keys
	numberOfNodes := len(dkgNodeIDStrings)
	publicKeys := make([]crypto.PublicKey, 0, numberOfNodes)
	for i := 0; i < numberOfNodes; i++ {
		privateKey := unittest.KeyFixture(crypto.BLSBLS12381)
		publicKeys = append(publicKeys, privateKey.PublicKey())
	}
	// create a group public key
	groupPublicKey := unittest.KeyFixture(crypto.BLSBLS12381).PublicKey()

	err := s.contractClient.SubmitResult(groupPublicKey, publicKeys)
	require.NoError(s.T(), err)
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
		valueNodeIDs = append(valueNodeIDs, cadence.NewString(nodeID.String()))
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

func (s *ClientSuite) createParticipant(nodeID flow.Identifier) {

	// create DKG partcipant
	createParticipantTx := sdk.NewTransaction().
		SetScript(templates.GenerateCreateDKGParticipantScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.blockchain.ServiceKey().Address, s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.dkgAddress)

	err := createParticipantTx.AddArgument(cadence.NewAddress(s.dkgAddress))
	require.NoError(s.T(), err)
	err = createParticipantTx.AddArgument(cadence.NewString(nodeID.String()))
	require.NoError(s.T(), err)

	s.signAndSubmit(createParticipantTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.dkgSigner},
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
	err := s.emulatorClient.Submit(tx)
	require.NoError(s.T(), err)
}

func (s *ClientSuite) executeScript(script []byte, arguments [][]byte) cadence.Value {

	// execute script
	result, err := s.blockchain.ExecuteScript(script, arguments)
	require.NoError(s.T(), err)
	require.True(s.T(), result.Succeeded())

	return result.Value
}
