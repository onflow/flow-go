package testnet

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/dapperlabs/testingdock"
	"github.com/dgraph-io/badger/v2"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bootstrapcmd "github.com/dapperlabs/flow-go/cmd/bootstrap/cmd"
	bootstraprun "github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/integration/client"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
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
	DefaultExecutionTrieDir = DefaultExecutionRootDir + "/triedb"
	DefaultExecutionDataDir = DefaultExecutionRootDir + "/valuedb"

	// ColNodeAPIPort is the name used for the collection node API port.
	ColNodeAPIPort = "col-ingress-port"
	// ExeNodeAPIPort is the name used for the execution node API port.
	ExeNodeAPIPort = "exe-api-port"
)

// FlowNetwork represents a test network of Flow nodes running in Docker containers.
type FlowNetwork struct {
	suite      *testingdock.Suite
	Network    *testingdock.Network
	Containers []*NodeContainer
}

func (n *FlowNetwork) Identities() flow.IdentityList {
	il := make(flow.IdentityList, 0, len(n.Containers))
	for _, c := range n.Containers {
		il = append(il, c.Identity)
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

	var merr *multierror.Error

	// remove data directories
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
func (n *FlowNetwork) ContainerByID(id flow.Identifier) (*NodeContainer, bool) {
	for _, c := range n.Containers {
		if c.Identity.NodeID == id {
			return c, true
		}
	}
	return nil, false
}

// NodeContainer represents a test Docker container for a generic Flow node.
type NodeContainer struct {
	*testingdock.Container
	Identity *flow.Identity    // the node identity
	Ports    map[string]string // port mapping
	DataDir  string            // host directory bound to container's database
	Opts     *testingdock.ContainerOpts
}

// bindPort exposes the given container port and binds it to the given host port.
// If no protocol is specified, assumes TCP.
func (c *NodeContainer) bindPort(hostPort, containerPort string) {

	// use TCP protocol if none specified
	containerNATPort := nat.Port(containerPort)
	if containerNATPort.Proto() == "" {
		containerNATPort = nat.Port(fmt.Sprintf("%s/tcp", containerPort))
	}

	c.Opts.Config.ExposedPorts = nat.PortSet{
		containerNATPort: {},
	}
	c.Opts.HostConfig.PortBindings = nat.PortMap{
		containerNATPort: []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: hostPort,
			},
		},
	}
}

// addFlag adds a command line flag to the container's startup command.
func (c *NodeContainer) addFlag(flag, val string) {
	c.Opts.Config.Cmd = append(
		c.Opts.Config.Cmd,
		fmt.Sprintf("--%s=%s", flag, val),
	)
}

// Name returns the container name. This is the name that appears in logs as
// well as the hostname that container can be reached at over the Docker network.
func (c *NodeContainer) Name() string {
	return c.Opts.Name
}

// DB returns the node's database.
func (c *NodeContainer) DB() (*badger.DB, error) {
	dbPath := filepath.Join(c.DataDir, DefaultFlowDBDir)
	db, err := badger.Open(badger.DefaultOptions(dbPath).WithLogger(nil))
	return db, err
}

// NodeConfig defines the config for a single node. This is used to start a
// container for the node.
type NodeConfig struct {
	Role                   flow.Role
	Stake                  uint64
	Identifier             flow.Identifier
	ContainerName          string
	NetworkKey             crypto.PrivateKey
	StakingKey             crypto.PrivateKey
	RandomBeaconKey        crypto.PrivateKey
	RandomBeaconGroupIndex int
}

// ImageName returns the Docker image name for the given config.
func (c *NodeConfig) ImageName() string {
	return fmt.Sprintf("gcr.io/dl-flow/%s:latest", c.Role.String())
}

func NewNodeConfig(role flow.Role, opts ...func(*NodeConfig)) *NodeConfig {
	c := &NodeConfig{
		Role:       role,
		Stake:      1000,                         // default stake
		Identifier: unittest.IdentifierFixture(), // default random ID
	}

	for _, apply := range opts {
		apply(c)
	}

	return c
}

// healthcheckGRPC returns a Docker healthcheck function that pings the GRPC
// service exposed at the given port.
func healthcheckGRPC(apiPort string) func() error {
	return func() error {
		fmt.Println("healthchecking...")
		c, err := client.New(fmt.Sprintf(":%s", apiPort))
		if err != nil {
			return err
		}

		return c.Ping(context.Background())
	}
}

// TODO consolidate with dupe from cmd/bootstrap
// getSeeds returns a list of n random seeds of 48 bytes each. This is used in
// conjunction with bootstrap key generation functions.
func getSeeds(n int) ([][]byte, error) {
	seeds := make([][]byte, n)
	for i := 0; i < n; i++ {
		seed := make([]byte, 48)
		_, err := rand.Read(seed)
		if err != nil {
			return nil, err
		}

		seeds[i] = seed
	}
	return seeds, nil
}

// TODO consolidate with dupe from cmd/bootstrap
// getQCSignerData packages up all the keys necessary in order to generate a
// quorum certificate for the genesis block.
//
// The input list of node configs and identities must have the same node at
// each index.
func getQCSignerData(groupPubKey crypto.PublicKey, nodes []*NodeConfig, identities flow.IdentityList) *bootstraprun.SignerData {

	// we need the DKG group index and signer info for each consensus node
	n := len(identities.Filter(filter.HasRole(flow.RoleConsensus)))
	dkgIndices := make([]int, 0, n)
	signers := make([]bootstraprun.Signer, 0, n)

	for i := 0; i < len(nodes); i++ {

		conf := nodes[i]
		identity := identities[i]
		if conf.Role != flow.RoleConsensus {
			continue
		}

		signer := bootstraprun.Signer{
			Identity:            *identity,
			StakingPrivKey:      conf.StakingKey,
			RandomBeaconPrivKey: conf.RandomBeaconKey,
		}
		signers = append(signers, signer)

		dkgIndex := conf.RandomBeaconGroupIndex
		dkgIndices = append(dkgIndices, dkgIndex)
	}

	dkgPubData := &hotstuff.DKGPublicData{
		GroupPubKey:           groupPubKey,
		IdToDKGParticipantMap: make(map[flow.Identifier]*hotstuff.DKGParticipant),
	}
	for i := 0; i < n; i++ {
		participant := signers[i]

		dkgPubData.IdToDKGParticipantMap[participant.Identity.NodeID] = &hotstuff.DKGParticipant{
			Id:             participant.Identity.NodeID,
			PublicKeyShare: participant.RandomBeaconPrivKey.PublicKey(),
			DKGIndex:       dkgIndices[i],
		}
	}

	signerData := &bootstraprun.SignerData{
		DkgPubData: dkgPubData,
		Signers:    signers,
	}
	return signerData
}

// currentUser returns a uid:gid Unix user identifier string for the current
// user. This is used to run node containers under the same user to avoid
// permission conflicts on files mounted from the host.
func currentUser() string {
	cur, _ := user.Current()
	return fmt.Sprintf("%s:%s", cur.Uid, cur.Gid)
}

func PrepareFlowNetwork(t *testing.T, name string, nodes []*NodeConfig) (*FlowNetwork, error) {

	if len(nodes) == 0 {
		return nil, fmt.Errorf("must specify at least one node")
	}

	// keep track of how many roles we have assigned so we can number containers
	// correctly (consensus_1, consensus_2, etc.)
	roleCounter := make(map[flow.Role]uint)

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

	// STEP 1 - GENERATE GENESIS BLOCK
	// Generate networking and staking keys for all nodes and determine their
	// Docker address. Run the DKG for all consensus nodes to generate random
	// beacon keys. Generate a genesis block including all the specified node
	// identities (and their keys) to bootstrap the protocol state and a QC for
	// the genesis block to

	// get networking keys for all nodes
	networkKeySeeds, err := getSeeds(len(nodes))
	require.Nil(t, err)
	networkKeys, err := bootstraprun.GenerateNetworkingKeys(len(nodes), networkKeySeeds)
	require.Nil(t, err)

	// get staking keys for all nodes
	stakingKeySeeds, err := getSeeds(len(nodes))
	require.Nil(t, err)
	stakingKeys, err := bootstraprun.GenerateStakingKeys(len(nodes), stakingKeySeeds)
	require.Nil(t, err)

	// create an identity for each specified node
	identities := make(flow.IdentityList, 0, len(nodes))
	for i, conf := range nodes {

		// define the node's name <role>_<n> and address <name>:<port>
		name := fmt.Sprintf("%s_%d", conf.Role.String(), roleCounter[conf.Role]+1)
		conf.ContainerName = name

		addr := fmt.Sprintf("%s:%d", name, 2137)
		roleCounter[conf.Role]++

		// add the public key to the node identity
		identity := &flow.Identity{
			NodeID:             conf.Identifier,
			Address:            addr,
			Role:               conf.Role,
			Stake:              conf.Stake,
			NetworkPubKey:      networkKeys[i].PublicKey(),
			StakingPubKey:      stakingKeys[i].PublicKey(),
			RandomBeaconPubKey: nil,
		}
		identities = append(identities, identity)

		// add the private key to the node config
		conf.NetworkKey = networkKeys[i]
		conf.StakingKey = stakingKeys[i]
	}

	conIdentities := identities.Filter(filter.HasRole(flow.RoleConsensus))

	// run DKG for all consensus nodes
	dkgSeeds, err := getSeeds(len(conIdentities))
	dkg, err := bootstraprun.RunDKG(len(conIdentities), dkgSeeds)
	require.Nil(t, err)

	// add the public key to the identity and the private key to the node config
	i := 0
	for _, conf := range nodes {
		if conf.Role != flow.RoleConsensus {
			continue
		}
		conf.RandomBeaconKey = dkg.Participants[i].Priv
		conf.RandomBeaconGroupIndex = dkg.Participants[i].GroupIndex
		identity, ok := conIdentities.ByNodeID(conf.Identifier)
		assert.True(t, ok)
		identity.RandomBeaconPubKey = dkg.PubKeys[i]
		i++
	}

	// generate genesis block
	seal := bootstraprun.GenerateRootSeal(flow.GenesisStateCommitment)
	genesis := bootstraprun.GenerateRootBlock(identities, seal)

	// generate QC
	signerData := getQCSignerData(dkg.PubGroupKey, nodes, identities)
	qc, err := bootstraprun.GenerateGenesisQC(*signerData, &genesis)
	require.Nil(t, err)

	// generate encodable DKG public data
	// TODO clean this up. There is a lot of duplication between this, DKG logic
	// in HotStuff, and DKG logic in cmd/bootstrap
	dkgPubData := bootstrapcmd.DKGDataPub{
		PubGroupKey: bootstrapcmd.EncodableRandomBeaconPubKey{
			PublicKey: dkg.PubGroupKey,
		},
	}

	// TODO duplicated from above, clean this up :(
	i = 0
	for _, conf := range nodes {
		if conf.Role != flow.RoleConsensus {
			continue
		}
		identity, ok := conIdentities.ByNodeID(conf.Identifier)
		assert.True(t, ok)

		// add the node to the DKG public data
		dkgPubData.Participants = append(dkgPubData.Participants,
			bootstrapcmd.DKGParticipantPub{
				NodeID: identity.NodeID,
				RandomBeaconPubKey: bootstrapcmd.EncodableRandomBeaconPubKey{
					PublicKey: identity.RandomBeaconPubKey,
				},
				GroupIndex: conf.RandomBeaconGroupIndex,
			})
	}

	// TODO private random beacon key files

	// create a temporary directory to store all bootstrapping files, these
	// will be shared between all nodes
	bootstrapDir, err := ioutil.TempDir(TmpRoot, "flow-integration-bootstrap")
	require.Nil(t, err)

	// write common genesis bootstrap files
	err = writeJSON(filepath.Join(bootstrapDir, bootstrapcmd.FilenameGenesisBlock), genesis)
	require.Nil(t, err)
	err = writeJSON(filepath.Join(bootstrapDir, bootstrapcmd.FilenameGenesisQC), qc)
	require.Nil(t, err)
	err = writeJSON(filepath.Join(bootstrapDir, bootstrapcmd.FilenameDKGDataPub), dkgPubData)
	require.Nil(t, err)

	// write keyfiles for each node
	for i := range nodes {
		conf := nodes[i]
		identity := identities[i]

		// TODO consolidate models
		writeable := bootstrapcmd.NodeInfoPriv{
			Role:           identity.Role,
			Address:        identity.Address,
			NodeID:         identity.NodeID,
			NetworkPrivKey: bootstrapcmd.EncodableNetworkPrivKey{conf.NetworkKey},
			StakingPrivKey: bootstrapcmd.EncodableStakingPrivKey{conf.StakingKey},
		}

		err = writeJSON(filepath.Join(bootstrapDir, fmt.Sprintf(bootstrapcmd.FilenameNodeInfoPriv, identity.NodeID)), writeable)
		require.Nil(t, err)
	}

	// STEP 2 - CREATE CONTAINERS

	containers := make([]*NodeContainer, len(nodes))
	for i := 0; i < len(nodes); i++ {
		conf := nodes[i]
		identity := identities[i]

		nodeContainer, err := createContainer(t, suite, bootstrapDir, conf, identity)
		require.Nil(t, err)
		network.After(nodeContainer.Container)

		containers[i] = nodeContainer
	}

	return &FlowNetwork{
		suite:      suite,
		Network:    network,
		Containers: containers,
	}, nil
}

// createContainer ...
func createContainer(t *testing.T, suite *testingdock.Suite, bootstrapDir string, conf *NodeConfig, identity *flow.Identity) (*NodeContainer, error) {
	opts := &testingdock.ContainerOpts{
		ForcePull: false,
		Name:      conf.ContainerName,
		Config: &container.Config{
			Image: conf.ImageName(),
			User:  currentUser(),
			Cmd: []string{
				//fmt.Sprintf("--entries=%s"), // TODO this should be from genesis
				fmt.Sprintf("--nodeid=%s", identity.NodeID.String()),
				fmt.Sprintf("--bootstrapdir=%s", DefaultBootstrapDir),
				fmt.Sprintf("--datadir=%s", DefaultFlowDBDir),
				"--loglevel=debug",
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
		return nil, fmt.Errorf("could not get tmp dir: %w", err)
	}

	nodeContainer := &NodeContainer{
		Identity: identity,
		Ports:    make(map[string]string),
		DataDir:  tmpdir,
		Opts:     opts,
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

	switch identity.Role {
	case flow.RoleCollection:

		hostPort := testingdock.RandomPort(t)
		containerPort := "9000/tcp"

		nodeContainer.bindPort(hostPort, containerPort)

		nodeContainer.addFlag("ingress-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
		nodeContainer.Opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckGRPC(hostPort))
		nodeContainer.Ports[ColNodeAPIPort] = hostPort

	case flow.RoleExecution:

		hostPort := testingdock.RandomPort(t)
		containerPort := "9000/tcp"

		nodeContainer.bindPort(hostPort, containerPort)

		nodeContainer.addFlag("rpc-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
		nodeContainer.Opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckGRPC(hostPort))
		nodeContainer.Ports[ExeNodeAPIPort] = hostPort

		// create directories for execution state trie and values in the tmp
		// host directory.
		exeDBTrieDir := filepath.Join(tmpdir, DefaultExecutionTrieDir)
		err = os.MkdirAll(exeDBTrieDir, 0700)
		require.Nil(t, err)

		exeDBValueDir := filepath.Join(tmpdir, DefaultExecutionDataDir)
		err = os.MkdirAll(exeDBValueDir, 0700)
		if err != nil {
			return nil, fmt.Errorf("could not create exe value dir: %w", err)
		}

		opts.HostConfig.Binds = append(
			opts.HostConfig.Binds,
			fmt.Sprintf("%s:%s:rw", exeDBTrieDir, DefaultExecutionTrieDir),
			fmt.Sprintf("%s:%s:rw", exeDBValueDir, DefaultExecutionDataDir),
		)

		nodeContainer.addFlag("triedir", DefaultExecutionRootDir)
	}

	suiteContainer := suite.Container(*opts)
	nodeContainer.Container = suiteContainer
	return nodeContainer, nil
}

func writeJSON(path string, data interface{}) error {
	marshaled, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, marshaled, 0644)
	return err
}
