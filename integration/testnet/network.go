package testnet

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-multierror"
	"github.com/m4ksio/testingdock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// DefaultDataDir is the default directory for the node database.
	DefaultDataDir = "/flow"

	// ColNodeAPIPort is the name used for the collection node API port.
	ColNodeAPIPort = "col-ingress-port"
	// ExeNodeAPIPort is the name used for the execution node API port.
	ExeNodeAPIPort = "exe-api-port"
)

// FlowNetwork represents a test network of Flow nodes running in Docker containers.
type FlowNetwork struct {
	suite      *testingdock.Suite
	Network    *testingdock.Network
	Containers []*FlowContainer
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
func (n *FlowNetwork) ContainerByID(id flow.Identifier) (*FlowContainer, bool) {
	for _, c := range n.Containers {
		if c.Identity.NodeID == id {
			return c, true
		}
	}
	return nil, false
}

// FlowContainer represents a test Docker container for a generic Flow node.
type FlowContainer struct {
	*testingdock.Container
	Identity flow.Identity     // the node identity
	Ports    map[string]string // port mapping
	DataDir  string            // host directory bound to container's database
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

type rolesCounts map[flow.Role]uint

// countRoles counts how many times each role occurs
func countRoles(identities []*NodeConfig) rolesCounts {
	ret := make(rolesCounts)

	for _, identity := range identities {
		ret[identity.Role] = ret[identity.Role] + 1
	}

	return ret
}

func healthcheckGRPC(context context.Context, apiPort string) error {
	fmt.Printf("healthchecking...\n")
	c, err := client.New("localhost:" + apiPort)
	if err != nil {
		return err
	}
	return c.Ping(context)
}

// imageName returns the canonical image name for the given role.
func imageName(role flow.Role) string {
	return fmt.Sprintf("gcr.io/dl-flow/%s:latest", role.String())
}

func PrepareFlowNetwork(context context.Context, t *testing.T, name string, nodes []*NodeConfig) (*FlowNetwork, error) {

	// count each role occurence
	identitiesCounts := countRoles(nodes)

	// counters for every role containers
	rolesCounters := rolesCounts{}

	opts := make([]*testingdock.ContainerOpts, len(nodes))
	identities := make([]*flow.Identity, len(nodes))
	identitiesStr := make([]string, len(nodes))
	containers := make([]*FlowContainer, len(nodes))
	var networkIdentities string

	suite, _ := testingdock.GetOrCreateSuite(t, name, testingdock.SuiteOpts{})

	// create network
	network := suite.Network(testingdock.NetworkOpts{
		Name: name,
	})

	// containerName assigns a name to a container - if there are multiple instances of the same role, suffix is added
	containerName := func(role flow.Role) string {
		identitiesCount := identitiesCounts[role]

		if identitiesCount == 1 {
			return role.String()
		}

		counter := rolesCounters[role]
		defer func() {
			rolesCounters[role] = rolesCounters[role] + 1
		}()

		return fmt.Sprintf("%s_%d", role.String(), counter)
	}

	assign := func(node *NodeConfig) (*testingdock.ContainerOpts, *flow.Identity) {

		name := containerName(node.Role)
		imageName := imageName(node.Role)

		opts := &testingdock.ContainerOpts{
			ForcePull: false,
			Name:      name,
			Config: &container.Config{
				Image: imageName,
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

	add := func(opts *testingdock.ContainerOpts, identity *flow.Identity) *FlowContainer {

		flowContainer := &FlowContainer{
			Identity: *identity,
			Ports:    make(map[string]string),
		}

		opts.Config.Cmd = []string{
			fmt.Sprintf("--entries=%s", networkIdentities),
			fmt.Sprintf("--nodeid=%s", identity.NodeID.String()),
			fmt.Sprintf("--datadir=%s", DefaultDataDir),
			"--loglevel=debug",
			"--nclusters=1",
		}

		// get a temporary directory in the host
		tmpdir, err := ioutil.TempDir("/tmp", "flow-integration")
		require.Nil(t, err)
		flowContainer.DataDir = tmpdir

		// Bind the host directory to the container's database directory
		// NOTE: I did this using the approach from:
		// https://github.com/fsouza/go-dockerclient/issues/132#issuecomment-50694902
		opts.HostConfig.Binds = append(
			opts.HostConfig.Binds,
			fmt.Sprintf("%s:%s:rw", tmpdir, DefaultDataDir),
		)

		switch identity.Role {
		// enhance with extras for collection node
		case flow.RoleCollection:

			// get a free host port for the collection node ingress grpc service
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

			opts.HealthCheck = testingdock.HealthCheckCustom(func() error {
				return healthcheckGRPC(context, apiPort)
			})

			flowContainer.Ports[ColNodeAPIPort] = apiPort

		case flow.RoleExecution:

			apiPort := testingdock.RandomPort(t)

			opts.Config.ExposedPorts = nat.PortSet{
				"9000/tcp": {},
			}
			opts.Config.Cmd = append(opts.Config.Cmd, fmt.Sprintf("--rpc-addr=%s:9000", opts.Name))
			opts.HostConfig = &container.HostConfig{
				PortBindings: nat.PortMap{
					"9000/tcp": []nat.PortBinding{
						{
							HostIP:   "0.0.0.0",
							HostPort: apiPort,
						},
					},
				},
			}
			opts.HealthCheck = testingdock.HealthCheckCustom(func() error {
				return healthcheckGRPC(context, apiPort)
			})

			flowContainer.Ports[ExeNodeAPIPort] = apiPort
		}

		c := suite.Container(*opts)

		network.After(c)
		flowContainer.Container = c

		return flowContainer
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
