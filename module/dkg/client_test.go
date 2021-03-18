package dkg

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	emulator "github.com/onflow/flow-emulator"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/messages"
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

// Setup Test creates the blockchain client, the emulated blockchain and deploys
// the DKG contract to the emulator
func (s *ClientSuite) SetupTest() {
	emulator, err := emulator.NewBlockchain()
	require.NoError(s.T(), err)

	s.emulator = emulator
	s.emulatorClient = NewEmulatorClient(emulator)

	key, signer := test.AccountKeyGenerator().NewWithSigner()
	address, err := s.emulator.CreateAccount([]*sdk.AccountKey{key}, []sdktemplates.Contract{})
	require.NoError(s.T(), err)

	s.client = NewClient(s.emulatorClient, signer, s.dkgAddress.String(), address.String(), 0)

	// deploy contract
	s.deployDKGContract()
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

func (s *ClientSuite) TestBroadcast() {
	err := s.client.Broadcast(messages.NewDKGMessage(0, []byte{}, ""))
	assert.NoError(s.T(), err)
}

func (s *ClientSuite) TestReadBroadcast() {
	messages, err := s.client.ReadBroadcast(0, unittest.IdentifierFixture())
	assert.NoError(s.T(), err)

	// TODO: check the return of the messages struct
	assert.True(s.T(), len(messages) == 0)
}

func (s *ClientSuite) TestSubmitResult() {
	numberOfKeys := 5

	// generate list of public keys
	publicKeys := make([]crypto.PublicKey, numberOfKeys)
	for i := 0; i < numberOfKeys; i++ {
		privateKey := unittest.KeyFixture(crypto.BLSBLS12381)
		publicKeys = append(publicKeys, privateKey.PublicKey())
	}

	// create a group public key
	groupPublicKey := unittest.KeyFixture(crypto.BLSBLS12381).PublicKey()

	err := s.client.SubmitResult(groupPublicKey, publicKeys)
	require.NoError(s.T(), err)
}
