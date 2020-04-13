package testnet

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/dapperlabs/testingdock"

	bootstrapcmd "github.com/dapperlabs/flow-go/cmd/bootstrap/cmd"
	bootstraprun "github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// TmpRoot is the default root directory to create temporary data
	// directories for containers. We use /tmp because $TMPDIR is not exposed
	// to docker by default on macOS
	TmpRoot = "/tmp"

	// DefaultBootstrapDir is the default directory for bootstrap files
	DefaultBootstrapDir = "/bootstrap"

	// DefaultFlowDBDir is the default directory for the node database.
	DefaultFlowDBDir = "/flowdb"
	// DefaultExecutionRootDir is the default directory for the execution node
	// state database.
	DefaultExecutionRootDir = "/exedb"

	// ColNodeAPIPort is the name used for the collection node API port.
	ColNodeAPIPort = "col-ingress-port"
	// ExeNodeAPIPort is the name used for the execution node API port.
	ExeNodeAPIPort = "exe-api-port"
	// AccessNodeAPIPort is the name used for the access node API port.
	AccessNodeAPIPort = "access-api-port"
)

func init() {
	testingdock.Verbose = true
}

// FlowNetwork represents a test network of Flow nodes running in Docker containers.
type FlowNetwork struct {
	suite       *testingdock.Suite
	Network     *testingdock.Network
	Containers  []*Container
	AccessPorts map[string]string
}

// Identities returns a list of identities, one for each node in the network.
func (n *FlowNetwork) Identities() flow.IdentityList {
	il := make(flow.IdentityList, 0, len(n.Containers))
	for _, c := range n.Containers {
		il = append(il, c.Config.Identity())
	}
	return il
}

// Start starts the network.
func (n *FlowNetwork) Start(ctx context.Context) {
	n.suite.Start(ctx)
}

// Stop stops the network and cleans up all resources. If you need to inspect
// state, first stop the containers, then check state, then clean up resources.
func (n *FlowNetwork) Stop() error {

	err := n.StopContainers()
	if err != nil {
		return fmt.Errorf("could not stop network: %w", err)
	}

	err = n.Cleanup()
	if err != nil {
		return fmt.Errorf("could not clean up network resources: %w", err)
	}

	return nil
}

// StopContainers spins down the network.
func (n *FlowNetwork) StopContainers() error {

	// stop the containers
	err := n.suite.Close()
	if err != nil {
		return fmt.Errorf("could not stop containers: %w", err)
	}

	return nil
}

// Cleanup cleans up all temporary files used by the network.
func (n *FlowNetwork) Cleanup() error {

	// remove data directories
	var merr *multierror.Error
	for _, c := range n.Containers {
		err := os.RemoveAll(c.DataDir)
		if err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	return merr.ErrorOrNil()
}

// ContainerByID returns the container with the given node ID. If such a
// container exists, returns true. Otherwise returns false.
func (n *FlowNetwork) ContainerByID(id flow.Identifier) (*Container, bool) {
	for _, c := range n.Containers {
		if c.Config.NodeID == id {
			return c, true
		}
	}
	return nil, false
}

// NetworkConfig is the config for the network.
// TODO add nclusters, etc. here
type NetworkConfig struct {
	Nodes []NodeConfig
}

func (n *NetworkConfig) Len() int {
	return len(n.Nodes)
}

func (n *NetworkConfig) Less(i, j int) bool {
	return n.Nodes[i].Role < n.Nodes[j].Role
}

func (n *NetworkConfig) Swap(i, j int) {
	n.Nodes[i], n.Nodes[j] = n.Nodes[j], n.Nodes[i]
}

// NodeConfig defines the input config for a particular node, specified prior
// to network creation.
type NodeConfig struct {
	Role       flow.Role
	Stake      uint64
	Identifier flow.Identifier
	LogLevel   string
}

func NewNodeConfig(role flow.Role, opts ...func(*NodeConfig)) NodeConfig {
	c := NodeConfig{
		Role:       role,
		Stake:      1000,                         // default stake
		Identifier: unittest.IdentifierFixture(), // default random ID
		LogLevel:   "debug",                      // log at debug by default
	}

	for _, apply := range opts {
		apply(&c)
	}

	return c
}

func WithID(id flow.Identifier) func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.Identifier = id
	}
}

// WithIDInt sets the node ID so the hex representation matches the input.
// Useful for having consistent and easily readable IDs in test logs.
func WithIDInt(id uint) func(config *NodeConfig) {

	idStr := strconv.Itoa(int(id))
	// left pad ID with zeros
	pad := strings.Repeat("0", 64-len(idStr))
	hex := pad + idStr

	// convert hex to ID
	flowID, err := flow.HexStringToIdentifier(hex)
	if err != nil {
		panic(err)
	}

	return WithID(flowID)
}

func WithLogLevel(level string) func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.LogLevel = level
	}
}

func PrepareFlowNetwork(t *testing.T, name string, networkConf NetworkConfig) (*FlowNetwork, error) {

	// number of nodes
	nNodes := len(networkConf.Nodes)

	if nNodes == 0 {
		return nil, fmt.Errorf("must specify at least one node")
	}

	// Sort so that access nodes start up last
	sort.Sort(&networkConf)

	// set up docker client
	dockerClient, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	require.Nil(t, err)

	suite, _ := testingdock.GetOrCreateSuite(t, name, testingdock.SuiteOpts{
		Client: dockerClient,
	})
	network := suite.Network(testingdock.NetworkOpts{
		Name: name,
	})

	// generate staking and networking keys for each configured node
	confs := setupKeys(t, networkConf)

	// run DKG for all consensus nodes
	dkg := runDKG(t, confs)

	// generate genesis block
	seal := bootstraprun.GenerateRootSeal(flow.GenesisStateCommitment)
	genesis := bootstraprun.GenerateRootBlock(toIdentityList(confs), seal)

	// generate QC
	nodeInfos := bootstrap.FilterByRole(toNodeInfoList(confs), flow.RoleConsensus)
	signerData := bootstrapcmd.GenerateQCParticipantData(nodeInfos, nodeInfos, dkg)
	qc, err := bootstraprun.GenerateGenesisQC(signerData, &genesis)
	require.Nil(t, err)

	// create a temporary directory to store all bootstrapping files, these
	// will be shared between all nodes
	bootstrapDir, err := ioutil.TempDir(TmpRoot, "flow-integration-bootstrap")
	require.Nil(t, err)

	// write common genesis bootstrap files
	err = writeJSON(filepath.Join(bootstrapDir, bootstrap.FilenameGenesisBlock), genesis)
	require.Nil(t, err)
	err = writeJSON(filepath.Join(bootstrapDir, bootstrap.FilenameGenesisQC), qc)
	require.Nil(t, err)
	err = writeJSON(filepath.Join(bootstrapDir, bootstrap.FilenameDKGDataPub), dkg.Public())
	require.Nil(t, err)

	// write private key files for each DKG participant
	for _, part := range dkg.Participants {
		filename := fmt.Sprintf(bootstrap.FilenameRandomBeaconPriv, part.NodeID)
		err = writeJSON(filepath.Join(bootstrapDir, filename), part.Private())
		require.Nil(t, err)
	}

	// write private key files for each node
	for _, nodeConfig := range confs {
		path := filepath.Join(bootstrapDir, fmt.Sprintf(bootstrap.FilenameNodeInfoPriv, nodeConfig.NodeID))

		// retrieve private representation of the node
		private, err := nodeConfig.NodeInfo.Private()
		require.Nil(t, err)

		err = writeJSON(path, private)
		require.Nil(t, err)
	}

	flowNetwork := &FlowNetwork{
		suite:       suite,
		Network:     network,
		Containers:  make([]*Container, 0, nNodes),
		AccessPorts: make(map[string]string),
	}
	// create container for each node
	for _, nodeConf := range confs {
		err := flowNetwork.createContainer(t, suite, bootstrapDir, nodeConf)
		require.Nil(t, err)
	}

	return flowNetwork, nil
}

// createContainer ...
func (f *FlowNetwork) createContainer(t *testing.T, suite *testingdock.Suite, bootstrapDir string, conf ContainerConfig) error {
	opts := &testingdock.ContainerOpts{
		ForcePull: false,
		Name:      conf.ContainerName,
		Config: &container.Config{
			Image: conf.ImageName(),
			User:  currentUser(),
			Cmd: []string{
				fmt.Sprintf("--nodeid=%s", conf.NodeID.String()),
				fmt.Sprintf("--bootstrapdir=%s", DefaultBootstrapDir),
				fmt.Sprintf("--datadir=%s", DefaultFlowDBDir),
				fmt.Sprintf("--loglevel=%s", conf.LogLevel),
				"--nclusters=1",
			},
		},
		HostConfig: &container.HostConfig{},
	}

	// get a temporary directory in the host. On macOS the default tmp
	// directory is NOT accessible to Docker by default, so we use /tmp
	// instead.
	tmpdir, err := ioutil.TempDir(TmpRoot, "flow-integration-node")
	if err != nil {
		return fmt.Errorf("could not get tmp dir: %w", err)
	}

	nodeContainer := &Container{
		Config:  conf,
		Ports:   make(map[string]string),
		DataDir: tmpdir,
		Opts:    opts,
	}

	// create a directory for the node database
	flowDBDir := filepath.Join(tmpdir, DefaultFlowDBDir)
	err = os.Mkdir(flowDBDir, 0700)
	require.Nil(t, err)

	// Bind the host directory to the container's database directory
	// Bind the common bootstrap directory to the container
	// NOTE: I did this using the approach from:
	// https://github.com/fsouza/go-dockerclient/issues/132#issuecomment-50694902
	opts.HostConfig.Binds = append(
		opts.HostConfig.Binds,
		fmt.Sprintf("%s:%s:rw", flowDBDir, DefaultFlowDBDir),
		fmt.Sprintf("%s:%s:ro", bootstrapDir, DefaultBootstrapDir),
	)

	switch conf.Role {
	case flow.RoleCollection:

		hostPort := testingdock.RandomPort(t)
		containerPort := "9000/tcp"

		nodeContainer.bindPort(hostPort, containerPort)

		nodeContainer.addFlag("ingress-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
		nodeContainer.Opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckAccessGRPC(hostPort))
		nodeContainer.Ports[ColNodeAPIPort] = hostPort
		f.AccessPorts[ColNodeAPIPort] = hostPort
	case flow.RoleExecution:

		hostPort := testingdock.RandomPort(t)
		containerPort := "9000/tcp"

		nodeContainer.bindPort(hostPort, containerPort)

		nodeContainer.addFlag("rpc-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
		nodeContainer.Opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckExecutionGRPC(hostPort))
		nodeContainer.Ports[ExeNodeAPIPort] = hostPort
		f.AccessPorts[ExeNodeAPIPort] = hostPort

		// create directories for execution state trie and values in the tmp
		// host directory.
		tmpLedgerDir, err := ioutil.TempDir(tmpdir, "flow-integration-trie")
		require.Nil(t, err)

		opts.HostConfig.Binds = append(
			opts.HostConfig.Binds,
			fmt.Sprintf("%s:%s:rw", tmpLedgerDir, DefaultExecutionRootDir),
		)

		nodeContainer.addFlag("triedir", DefaultExecutionRootDir)
	}

	suiteContainer := suite.Container(*opts)
	nodeContainer.Container = suiteContainer
	f.Containers = append(f.Containers, nodeContainer)
	f.Network.After(nodeContainer.Container)
	return nil
}

// setupKeys generates private staking and networking keys for each configured
// node. It also assigns each node a unique container name and network address.
func setupKeys(t *testing.T, networkConf NetworkConfig) []ContainerConfig {

	nNodes := len(networkConf.Nodes)

	// keep track of how many roles we have assigned so we can number containers
	// correctly (consensus_1, consensus_2, etc.)
	roleCounter := make(map[flow.Role]int)

	// get networking keys for all nodes
	networkKeys, err := unittest.NetworkingKeys(nNodes)
	require.Nil(t, err)

	// get staking keys for all nodes
	stakingKeys, err := unittest.StakingKeys(nNodes)
	require.Nil(t, err)

	// create node container configs and corresponding public identities
	confs := make([]ContainerConfig, 0, nNodes)
	for i, conf := range networkConf.Nodes {

		// define the node's name <role>_<n> and address <name>:<port>
		name := fmt.Sprintf("%s_%d", conf.Role.String(), roleCounter[conf.Role]+1)

		addr := fmt.Sprintf("%s:%d", name, 2137)
		roleCounter[conf.Role]++

		info := bootstrap.NewPrivateNodeInfo(
			conf.Identifier,
			conf.Role,
			addr,
			conf.Stake,
			networkKeys[i],
			stakingKeys[i],
		)

		containerConf := ContainerConfig{
			NodeInfo:      info,
			ContainerName: name,
			LogLevel:      conf.LogLevel,
		}

		confs = append(confs, containerConf)
	}

	return confs
}

func runDKG(t *testing.T, confs []ContainerConfig) bootstrap.DKGData {

	// filter by consensus nodes
	consensusNodes := bootstrap.FilterByRole(toNodeInfoList(confs), flow.RoleConsensus)
	nConsensusNodes := len(consensusNodes)

	// run the core dkg algorithm
	dkgSeeds, err := getSeeds(nConsensusNodes)
	require.Nil(t, err)
	dkg, err := bootstraprun.RunDKG(nConsensusNodes, dkgSeeds)
	require.Nil(t, err)

	// sanity check
	assert.Equal(t, nConsensusNodes, len(dkg.Participants))

	// set the node IDs in the dkg data
	for i := range dkg.Participants {
		nodeID := consensusNodes[i].NodeID
		dkg.Participants[i].NodeID = nodeID
	}

	return dkg
}
