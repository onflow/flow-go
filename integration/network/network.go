package network

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/m4ksio/testingdock"

	"github.com/dapperlabs/flow-go/utils/unittest"

	"github.com/dapperlabs/flow-go/model/flow"
)

type FlowNetwork struct {
	Suite      *testingdock.Suite
	Network    *testingdock.Network
	Containers []*FlowContainer
}

type FlowContainer struct {
	*testingdock.Container
	Identity flow.Identity
	Ports    map[string]string
}

type CollectionContainer struct {
	FlowContainer
	APIPort string
}

type FlowNode struct {
	Role       flow.Role
	Stake      uint64
	Identifier flow.Identifier
}

type rolesCounts map[flow.Role]uint

// countRoles counts how many times each role occurs
func countRoles(identities []*FlowNode) rolesCounts {
	ret := make(rolesCounts)

	for _, identity := range identities {
		ret[identity.Role] = ret[identity.Role] + 1
	}

	return ret
}

func identifier(identifier flow.Identifier) flow.Identifier {
	// Substitute magic zero value for random on
	if identifier == flow.ZeroID {
		return unittest.IdentifierFixture()
	}
	return identifier
}

func healthcheckGRPC(context context.Context, apiPort string) error {
	fmt.Printf("healthchecking...\n")
	c, err := client.New("localhost:" + apiPort)
	if err != nil {
		return err
	}
	return c.Ping(context)
}

func PrepareFlowNetwork(context context.Context, t *testing.T, name string, nodes []*FlowNode) (*FlowNetwork, error) {

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

	imageName := func(role flow.Role) string {
		return fmt.Sprintf("gcr.io/dl-flow/%s:latest", role.String())
	}

	assign := func(node *FlowNode) (*testingdock.ContainerOpts, *flow.Identity) {

		name := containerName(node.Role)
		imageName := imageName(node.Role)

		opts := &testingdock.ContainerOpts{
			ForcePull: false,
			Name:      name,
			Config: &container.Config{
				Image: imageName,
			},
		}

		identity := flow.Identity{
			NodeID:  identifier(node.Identifier),
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
			"--loglevel=debug",
		}

		switch identity.Role {

		// enhance with extras for collection node
		case flow.RoleCollection:

			collectionNodeAPIPort := testingdock.RandomPort(t)

			opts.Config.ExposedPorts = nat.PortSet{
				"9000/tcp": struct{}{},
			}
			opts.Config.Cmd = append(opts.Config.Cmd, fmt.Sprintf("--ingress-addr=%s:9000", opts.Name))
			opts.HostConfig = &container.HostConfig{
				PortBindings: nat.PortMap{
					nat.Port("9000/tcp"): []nat.PortBinding{
						{
							HostIP:   "0.0.0.0",
							HostPort: collectionNodeAPIPort,
						},
					},
				},
			}
			opts.HealthCheck = testingdock.HealthCheckCustom(func() error {
				return healthcheckGRPC(context, collectionNodeAPIPort)
			})

			flowContainer.Ports["api"] = collectionNodeAPIPort


		case flow.RoleExecution:

			executionNodeAPIPort := testingdock.RandomPort(t)

			opts.Config.ExposedPorts = nat.PortSet{
				"9000/tcp": struct{}{},
			}
			opts.Config.Cmd = append(opts.Config.Cmd, fmt.Sprintf("--rpc-addr=%s:9000", opts.Name))
			opts.HostConfig = &container.HostConfig{
				PortBindings: nat.PortMap{
					nat.Port("9000/tcp"): []nat.PortBinding{
						{
							HostIP:   "0.0.0.0",
							HostPort: executionNodeAPIPort,
						},
					},
				},
			}
			opts.HealthCheck = testingdock.HealthCheckCustom(func() error {
				return healthcheckGRPC(context, executionNodeAPIPort)
			})

			flowContainer.Ports["api"] = executionNodeAPIPort


		}

		c := suite.Container(*opts)

		network.After(c)
		flowContainer.Container = c

		return flowContainer
	}

	// assigns names and addresses, those depends on numbers of each service
	for i, node := range nodes {
		ops, identity := assign(node)

		opts[i] = ops
		identities[i] = identity
		identitiesStr[i] = identity.String()
	}

	networkIdentities = strings.Join(identitiesStr, ",")

	for i := range nodes {

		c := add(opts[i], identities[i])

		containers[i] = c
	}

	return &FlowNetwork{
		Suite:      suite,
		Network:    network,
		Containers: containers,
	}, nil
}
