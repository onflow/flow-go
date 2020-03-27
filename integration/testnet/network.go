package testnet

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dapperlabs/testingdock"
	"github.com/dgraph-io/badger/v2"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/client"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// TmpRoot is the default root directory to create temporary data
	// directories for containers. We use /tmp because $TMPDIR is not exposed
	// to docker by default on macOS
	TmpRoot = "/tmp"

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
		il = append(il, &c.Identity)
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
	Identity flow.Identity     // the node identity
	Ports    map[string]string // port mapping
	DataDir  string            // host directory bound to container's database
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
	Role       flow.Role
	Stake      uint64
	Identifier flow.Identifier
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

func PrepareFlowNetwork(t *testing.T, name string, nodes []*NodeConfig) (*FlowNetwork, error) {

	// get the current user so we can run the Docker container under the same
	// user and avoid permission conflicts
	currentUser, _ := user.Current()

	// keep track of how many role we have assigned so we can number containers
	// correctly (consensus_1, consensus_2, etc.)
	roleCounter := make(map[flow.Role]uint)

	opts := make([]*testingdock.ContainerOpts, len(nodes))
	identities := make([]*flow.Identity, len(nodes))
	identitiesStr := make([]string, len(nodes))
	containers := make([]*NodeContainer, len(nodes))
	var networkIdentities string

	suite, _ := testingdock.GetOrCreateSuite(t, name, testingdock.SuiteOpts{})

	// create network
	network := suite.Network(testingdock.NetworkOpts{
		Name: name,
	})

	// Assigns a configured node to a container name and returns options
	// to start the container
	assign := func(node *NodeConfig) (*testingdock.ContainerOpts, *flow.Identity) {

		n := roleCounter[node.Role] + 1
		name := fmt.Sprintf("%s_%d", node.Role.String(), n)
		imageName := fmt.Sprintf("gcr.io/dl-flow/%s:latest", node.Role.String())
		roleCounter[node.Role]++

		opts := &testingdock.ContainerOpts{
			ForcePull: false,
			Name:      name,
			Config: &container.Config{
				Image: imageName,
				User:  fmt.Sprintf("%s:%s", currentUser.Uid, currentUser.Gid),
			},
			HostConfig: &container.HostConfig{},
		}

		identity := flow.Identity{
			NodeID:  node.Identifier,
			Address: fmt.Sprintf("%s:%d", name, 2137),
			Role:    node.Role,
			Stake:   node.Stake,
		}

		return opts, &identity
	}

	add := func(opts *testingdock.ContainerOpts, identity *flow.Identity) *NodeContainer {

		container := &NodeContainer{
			Identity: *identity,
			Ports:    make(map[string]string),
		}

		opts.Config.Cmd = []string{
			fmt.Sprintf("--entries=%s", networkIdentities),
			fmt.Sprintf("--nodeid=%s", identity.NodeID.String()),
			fmt.Sprintf("--datadir=%s", DefaultFlowDBDir),
			"--loglevel=debug",
			"--nclusters=1",
		}

		// get a temporary directory in the host. On macOS the default tmp
		// directory is NOT accessible to Docker by default, so we use /tmp
		// instead.
		tmpdir, err := ioutil.TempDir(TmpRoot, "flow-integration")
		require.Nil(t, err)
		container.DataDir = tmpdir

		// create a directory for the node database
		flowDBDir := filepath.Join(tmpdir, DefaultFlowDBDir)
		err = os.Mkdir(flowDBDir, 0700)
		require.Nil(t, err)

		// Bind the host directory to the container's database directory
		// NOTE: I did this using the approach from:
		// https://github.com/fsouza/go-dockerclient/issues/132#issuecomment-50694902
		opts.HostConfig.Binds = append(
			opts.HostConfig.Binds,
			fmt.Sprintf("%s:%s:rw", flowDBDir, DefaultFlowDBDir),
		)

		switch identity.Role {
		case flow.RoleCollection:

			apiPort := testingdock.RandomPort(t)

			opts.Config.ExposedPorts = nat.PortSet{
				"9000/tcp": {},
			}
			opts.Config.Cmd = append(opts.Config.Cmd, fmt.Sprintf("--ingress-addr=%s:9000", opts.Name))
			opts.HostConfig.PortBindings = nat.PortMap{
				"9000/tcp": []nat.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: apiPort,
					},
				},
			}

			opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckGRPC(apiPort))
			container.Ports[ColNodeAPIPort] = apiPort

		case flow.RoleExecution:

			apiPort := testingdock.RandomPort(t)
			opts.Config.ExposedPorts = nat.PortSet{
				"9000/tcp": {},
			}

			// create directories for execution state trie and values in the tmp
			// host directory.
			exeDBTrieDir := filepath.Join(tmpdir, DefaultExecutionTrieDir)
			err = os.MkdirAll(exeDBTrieDir, 0700)
			require.Nil(t, err)

			exeDBValueDir := filepath.Join(tmpdir, DefaultExecutionDataDir)
			err = os.MkdirAll(exeDBValueDir, 0700)
			require.Nil(t, err)

			opts.Config.Cmd = append(opts.Config.Cmd,
				fmt.Sprintf("--rpc-addr=%s:9000", opts.Name),
				fmt.Sprintf("--triedir=%s", DefaultExecutionRootDir),
			)
			opts.HostConfig.PortBindings = nat.PortMap{
				"9000/tcp": []nat.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: apiPort,
					},
				},
			}
			opts.HostConfig.Binds = append(
				opts.HostConfig.Binds,
				fmt.Sprintf("%s:%s:rw", exeDBTrieDir, DefaultExecutionTrieDir),
				fmt.Sprintf("%s:%s:rw", exeDBValueDir, DefaultExecutionDataDir),
			)
			opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckGRPC(apiPort))
			container.Ports[ExeNodeAPIPort] = apiPort
		}

		c := suite.Container(*opts)

		network.After(c)
		container.Container = c

		return container
	}

	// assigns names and addresses, those depends on numbers of each service
	for i, node := range nodes {
		containerOpts, identity := assign(node)

		opts[i] = containerOpts
		identities[i] = identity
		identitiesStr[i] = identity.String()
	}

	networkIdentities = strings.Join(identitiesStr, ",")

	for i := range nodes {

		c := add(opts[i], identities[i])

		containers[i] = c
	}

	return &FlowNetwork{
		suite:      suite,
		Network:    network,
		Containers: containers,
	}, nil
}
