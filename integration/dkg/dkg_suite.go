package dkg

import (
	"fmt"

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
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/dkg"
	emulatormod "github.com/onflow/flow-go/module/emulator"
	"github.com/onflow/flow-go/network/stub"
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

	nodeIDs      flow.IdentityList
	nodeAccounts []*nodeAccount
	nodes        []*node
}

func (s *DKGSuite) SetupTest() {
	s.initEmulator()
	s.deployDKGContract()
	s.setupDKGAdmin()

	s.nodeIDs = unittest.IdentityListFixture(numberOfNodes, unittest.WithRole(flow.RoleConsensus))
	for _, id := range s.nodeIDs {
		s.nodeAccounts = append(s.nodeAccounts, s.createAndFundAccount(id.NodeID))
	}
	for _, acc := range s.nodeAccounts {
		s.nodes = append(s.nodes, s.createNode(acc))
	}

	s.startDKGWithParticipants(s.nodeIDs.NodeIDs())

	for _, node := range s.nodes {
		s.claimDKGParticipant(node)
	}
}

// initEmulator initializes the emulator and the admin emulator client
func (s *DKGSuite) initEmulator() {
	s.chainID = flow.Testnet

	blockchain, err := emulator.NewBlockchain()
	require.NoError(s.T(), err)

	s.blockchain = blockchain

	s.adminEmulatorClient = emulatormod.NewEmulatorClient(blockchain)
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
		s.adminEmulatorClient,
		s.dkgSigner,
		s.dkgAddress.String(),
		s.dkgAddress.String(), 0)
}

// setupDKGAdmin does XXX?
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
	s.signAndSubmit(setUpAdminTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.dkgSigner},
	)
}

// createAndFundAccount creates a nodeAccount and funds it in the emulator
func (s *DKGSuite) createAndFundAccount(netID flow.Identifier) *nodeAccount {
	accountPrivateKey := common.RandomPrivateKey()
	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetSigAlgo(sdkcrypto.ECDSA_P256).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)
	accountSigner := sdkcrypto.NewInMemorySigner(accountPrivateKey, accountKey.HashAlgo)

	/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	create Flow account
	~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

	createAccountTx := sdktemplates.CreateAccount(
		[]*sdk.AccountKey{accountKey},
		[]sdktemplates.Contract{},
		s.blockchain.ServiceKey().Address).
		SetProposalKey(
			s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index,
			s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address)

	s.signAndSubmit(createAccountTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer()},
	)

	res, err := s.adminEmulatorClient.GetTransactionResult(nil, createAccountTx.ID())
	require.NoError(s.T(), err)

	var newAccountAddress sdk.Address
	for _, event := range res.Events {
		if event.Type == sdk.EventAccountCreated {
			accountCreatedEvent := sdk.AccountCreatedEvent(event)
			newAccountAddress = accountCreatedEvent.Address()
		}
	}

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

	err = fundAccountTx.AddArgument(cadence.UFix64(1_0000_0000))
	require.NoError(s.T(), err)
	err = fundAccountTx.AddArgument(cadence.NewAddress(newAccountAddress))
	require.NoError(s.T(), err)

	s.signAndSubmit(fundAccountTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer()},
	)

	/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	create nodeAccount
	~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

	account := &nodeAccount{
		netID:          netID,
		privKey:        accountPrivateKey,
		accountKey:     accountKey,
		accountAddress: newAccountAddress,
		accountSigner:  accountSigner,
		accountInfo: &bootstrap.NodeMachineAccountInfo{
			Address:           newAccountAddress.String(),
			EncodedPrivateKey: accountKey.PublicKey.Encode(),
			KeyIndex:          0,
			SigningAlgorithm:  accountKey.SigAlgo,
			HashAlgorithm:     accountKey.HashAlgo,
		},
	}

	return account
}

// createNode creates a DKG test node from an account and initializes it's DKG
// smart-contract client
func (s *DKGSuite) createNode(account *nodeAccount) *node {
	emulatorClient := emulatormod.NewEmulatorClient(s.blockchain)
	contractClient := dkg.NewClient(
		emulatorClient,
		account.accountSigner,
		s.dkgAddress.String(),
		account.accountAddress.String(),
		0,
	)
	return &node{
		account:           account,
		dkgContractClient: contractClient,
	}
}

func (s *DKGSuite) startDKGWithParticipants(nodeIDs []flow.Identifier) {
	// convert node identifiers to candece.Value to be passed in as TX argument
	valueNodeIDs := make([]cadence.Value, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		valueNodeIDs = append(valueNodeIDs, cadence.NewString(nodeID.String()))
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

	s.signAndSubmit(startDKGTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.dkgSigner},
	)

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
	err = createParticipantTx.AddArgument(cadence.NewString(node.account.netID.String()))
	require.NoError(s.T(), err)

	s.signAndSubmit(createParticipantTx,
		[]sdk.Address{node.account.accountAddress, s.blockchain.ServiceKey().Address, s.dkgAddress},
		[]sdkcrypto.Signer{node.account.accountSigner, s.blockchain.ServiceKey().Signer(), s.dkgSigner},
	)

	// verify that nodeID was registered
	result := s.executeScript(templates.GenerateGetDKGNodeIsRegisteredScript(s.env),
		[][]byte{jsoncdc.MustEncode(cadence.String(node.account.netID.String()))})
	assert.Equal(s.T(), cadence.NewBool(true), result)
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// signAndSubmit commits a transaction
func (s *DKGSuite) signAndSubmit(tx *sdk.Transaction, signerAddresses []sdk.Address, signers []sdkcrypto.Signer) {

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
	err := s.adminEmulatorClient.Submit(tx)
	require.NoError(s.T(), err)
}

// executeScript runs a cadence script on the emulator blockchain
func (s *DKGSuite) executeScript(script []byte, arguments [][]byte) cadence.Value {
	result, err := s.blockchain.ExecuteScript(script, arguments)
	require.NoError(s.T(), err)
	require.True(s.T(), result.Succeeded())
	return result.Value
}
