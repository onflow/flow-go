package cohort3

import (
	"context"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/onflow/flow-go/utils/dsl"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTxValidator(t *testing.T) {
	suite.Run(t, new(TransactionValidatorTestSuite))
}

type TransactionValidatorTestSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *TransactionValidatorTestSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *TransactionValidatorTestSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel),
			testnet.WithAdditionalFlag("--check-payer-balance=true")),
	}

	// need one execution node
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel))
	exeConfig2 := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, exeConfig, exeConfig2)

	// need one dummy verification node
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one controllable collection node
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithAdditionalFlag("--hotstuff-proposal-duration=100ms"))
	nodeConfigs = append(nodeConfigs, collConfig)

	// need three consensus nodes
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithID(conID))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// set up network
	var flowNetworkOptions []testnet.NetworkConfigOpt
	flowNetworkOptions = append(flowNetworkOptions, testnet.WithClusters(1))

	conf := testnet.NewNetworkConfig("transaction_validator_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

func (s *TransactionValidatorTestSuite) TestHappyPath() {
	// get the access node container and client
	accessContainer := s.net.ContainerByName(testnet.PrimaryAN)
	client, err := accessContainer.TestnetClient()
	require.NoError(s.T(), err, "failed to get access node client")
	require.NotNil(s.T(), client, "failed to get access node client")

	// create account
	keys := test.AccountKeyGenerator()
	bobAccountKey, bobAccountSigner := keys.NewWithSigner()

	latestBlockID, err := client.GetLatestBlockID(s.ctx)
	s.Require().NoError(err)

	bobAccountAddress, err := client.CreateAccount(s.ctx, bobAccountKey, sdk.Identifier(latestBlockID))
	s.Require().NoError(err)

	//TODO: remove this. it's for debug only
	bobAccount, err := client.GetAccount(bobAccountAddress)
	s.Require().NoError(err, "failed to get account")
	bobAccountBalance := bobAccount.Balance

	// do the following steps on behalf of the account
	//
	// deploy contract to account
	//
	// 1.
	// - send any deployContractTx
	// - check that validator accepts deployContractTx
	//
	// 2.
	// - spend some flow
	// - send heavy deployContractTx
	// - check that validator rejects deployContractTx

	// deploy contract to account
	header, err := client.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	contract := dsl.Contract{
		Name: "TestContract",
		Members: []dsl.CadenceCode{
			dsl.Code(`
					access(all) var magicValue: Int32
	
					init() {
						self.magicValue = 0				
					}
	
					access(all) fun SetMagicValue(newValue: Int32) {
						self.magicValue = newValue			
					}
			`),
		},
	}

	deployContractDslTx := dsl.Transaction{
		Import: dsl.Import{},
		Content: dsl.Prepare{
			Content: dsl.SetAccountCode{
				Code: contract.ToCadence(),
				Name: contract.Name,
			},
		},
	}

	deployContractTx := sdk.NewTransaction().
		SetScript([]byte(deployContractDslTx.ToCadence())).
		SetReferenceBlockID(header.ID).
		SetProposalKey(bobAccountAddress, 0, 0).
		SetPayer(bobAccountAddress).
		AddAuthorizer(bobAccountAddress)

	err = deployContractTx.SignEnvelope(bobAccountAddress, 0, bobAccountSigner)
	s.Require().NoError(err)

	err = client.SendTransaction(s.ctx, deployContractTx)
	s.Require().NoError(err)

	deployTxRes, err := client.WaitForSealed(s.ctx, deployContractTx.ID())
	s.Require().NoError(err)
	s.Require().NotEmpty(deployTxRes)

	// execute some tx on behalf of account
	setValueTxDsl := dsl.Transaction{
		Import: dsl.Import{
			Names:   []string{"TestContract"},
			Address: bobAccountAddress,
		},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				TestContract.SetMagicValue(newValue: 42)
			`),
		},
	}

	header, err = client.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	setValueTx := sdk.NewTransaction().
		SetScript([]byte(setValueTxDsl.ToCadence())).
		SetComputeLimit(9999).
		SetReferenceBlockID(header.ID).
		SetProposalKey(bobAccountAddress, 0, 1).
		SetPayer(bobAccountAddress).
		AddAuthorizer(bobAccountAddress)

	err = setValueTx.SignEnvelope(bobAccountAddress, 0, bobAccountSigner)
	s.Require().NoError(err)

	err = client.SendTransaction(s.ctx, setValueTx)
	s.Require().NoError(err)

	setValueTxRes, err := client.WaitForSealed(s.ctx, setValueTx.ID())
	s.Require().NoError(err)
	s.Require().NotEmpty(setValueTxRes)

	// create acc (have to pay fee for this)
	keys = test.AccountKeyGenerator()
	aliceAccountKey, _ := keys.NewWithSigner()

	header, err = client.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	createAccountTx, err := templates.CreateAccount([]*sdk.AccountKey{aliceAccountKey}, nil, bobAccountAddress)
	s.Require().NoError(err)

	createAccountTx.
		SetComputeLimit(1000).
		SetReferenceBlockID(header.ID).
		SetProposalKey(bobAccountAddress, 0, 2).
		SetPayer(bobAccountAddress)

	err = createAccountTx.SignEnvelope(bobAccountAddress, 0, bobAccountSigner)
	s.Require().NoError(err)

	err = client.SendTransaction(s.ctx, createAccountTx)
	s.Require().NoError(err)

	createAccountTxRes, err := client.WaitForSealed(s.ctx, setValueTx.ID())
	s.Require().NoError(err)
	s.Require().NotEmpty(createAccountTxRes)

	//TODO: remove this. it's for debug only
	bobAccount, err = client.GetAccount(bobAccountAddress)
	s.Require().NoError(err, "failed to get account")
	newBobAccountBalance := bobAccount.Balance
	s.Require().NotEqual(bobAccountBalance, newBobAccountBalance)
}
