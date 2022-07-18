package access

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/cmd/bootstrap/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

func TestAccess(t *testing.T) {
	suite.Run(t, new(AccessSuite))
}

type AccessSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *AccessSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (suite *AccessSuite) SetupTest() {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "api.go").
		Str("testcase", suite.T().Name()).
		Logger()
	suite.log = logger
	suite.log.Info().Msg("================> SetupTest")
	defer func() {
		suite.log.Info().Msg("================> Finish SetupTest")
	}()

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel)),
	}

	// need one dummy execution node (unused ghost)
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one dummy verification node (unused ghost)
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one controllable collection node (unused ghost)
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, collConfig)

	// need three consensus nodes (unused ghost)
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithID(conID),
			testnet.AsGhost())
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	conf := testnet.NewNetworkConfig("access_api_test", nodeConfigs)
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf, flow.Localnet)

	// start the network
	suite.T().Logf("starting flow network with docker containers")
	suite.ctx, suite.cancel = context.WithCancel(context.Background())

	// observer node
	observerName := "observer_1"
	accessName := "access_1"
	accessPort := suite.net.AccessPorts["access-api-port"]
	accessPublicKey := hex.EncodeToString(suite.net.BootstrapData.StakedConfs[6].NetworkPubKey().Encode())

	writeObserverPrivateKey := func(observerName string) {
		// make the observer private key for named observer
		// only used for localnet, not for use with production
		networkSeed := cmd.GenerateRandomSeed(crypto.KeyGenSeedMinLenECDSASecp256k1)
		networkKey, err := utils.GeneratePublicNetworkingKey(networkSeed)
		if err != nil {
			panic(err)
		}

		// hex encode
		keyBytes := networkKey.Encode()
		output := make([]byte, hex.EncodedLen(len(keyBytes)))
		hex.Encode(output, keyBytes)

		// write to file
		outputFile := fmt.Sprintf("%s/private-root-information/%s_key", suite.net.BootstrapDir, observerName)
		err = ioutil.WriteFile(outputFile, output, 0600)
		if err != nil {
			panic(err)
		}
	}

	writeObserverPrivateKey(observerName)

	suite.net.AddContainer(suite.ctx, observerName, &container.Config{
		Image: "gcr.io/flow-container-registry/observer:latest",
		Cmd: []string{
			fmt.Sprintf("--bootstrap-node-addresses=%s:%s", accessName, accessPort),
			fmt.Sprintf("--bootstrap-node-public-keys=%s", accessPublicKey),
			fmt.Sprintf("--observer-networking-key-path=/bootstrap/private-root-information/%s_key", observerName),
			fmt.Sprintf("--bind=0.0.0.0:0"),
			fmt.Sprintf("--rpc-addr=%s:%s", observerName, "9000"),
			fmt.Sprintf("--secure-rpc-addr=%s:%s", observerName, "9001"),
			fmt.Sprintf("--http-addr=%s:%s", observerName, "8000"),
			"--bootstrapdir=/bootstrap",
			"--datadir=/data/protocol",
			"--secretsdir=/data/secrets",
			"--loglevel=DEBUG",
			fmt.Sprintf("--profiler-enabled=%t", false),
			fmt.Sprintf("--tracer-enabled=%t", false),
			"--profiler-dir=/profiler",
			"--profiler-interval=2m",
		},
	})

	fmt.Println(accessPublicKey)
	suite.net.Start(suite.ctx)
}

func (suite *AccessSuite) TestHTTPProxyPortOpen() {
	httpProxyAddress := fmt.Sprintf(":%s", suite.net.AccessPorts[testnet.AccessNodeAPIProxyPort])
	conn, err := net.DialTimeout("tcp", httpProxyAddress, 1*time.Second)
	require.NoError(suite.T(), err, "http proxy port not open on the access node")
	conn.Close()
}

func (suite *AccessSuite) TestAccessConnection() {
	t := suite.T()
	addr := "0.0.0.0:" + suite.net.AccessPorts["access-api-port"]

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Failed()
	}

	client := accessproto.NewAccessAPIClient(conn)

	_, err = client.Ping(suite.ctx, &accessproto.PingRequest{})
	if err != nil {
		t.Failed()
	}
}
