package network

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/m4ksio/testingdock"

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
	Identifier *flow.Identifier
}

type rolesCounts struct {
	collectionCounter   uint
	consensusCounter    uint
	executionCounter    uint
	verificationCounter uint
}

func countRoles(identities []*FlowNode) (rolesCounts, error) {
	ret := rolesCounts{}

	for _, identity := range identities {
		var counter *uint
		counter, err := ret.toCounter(identity.Role)
		if err != nil {
			return rolesCounts{}, err
		}
		*counter = (*counter) + 1
	}

	return ret, nil
}

func (r *rolesCounts) toCounter(role flow.Role) (*uint, error) {
	var counter *uint
	switch role {
	case flow.RoleConsensus:
		counter = &r.consensusCounter
	case flow.RoleCollection:
		counter = &r.collectionCounter
	case flow.RoleExecution:
		counter = &r.executionCounter
	case flow.RoleVerification:
		counter = &r.verificationCounter
	default:
		return nil, fmt.Errorf("unknown role %d", role)
	}
	return counter, nil
}

func identifier(identifier *flow.Identifier) flow.Identifier {
	if identifier == nil {
		return unittest.IdentifierFixture()
	}
	return *identifier
}

func healthcheckGRPC(apiPort string, context context.Context) error {
	fmt.Printf("healthchecking...\n")
	c, err := client.New("localhost:" + apiPort)
	if err != nil {
		return err
	}
	return c.Ping(context)
}

func PrepareFlowNetwork(t *testing.T, name string, context context.Context, nodes []*FlowNode) (*FlowNetwork, error) {

	// count each role occurence
	identitiesCounts, err := countRoles(nodes)
	if err != nil {
		return nil, err
	}

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

	containerName := func(role flow.Role) (string, error) {
		identitiesCount, err := identitiesCounts.toCounter(role)
		if err != nil {
			return "", fmt.Errorf("cannot get container name: %w", err)
		}

		if *identitiesCount == 1 {
			return role.String(), nil
		}

		counter, err := rolesCounters.toCounter(role)
		if err != nil {
			return "", fmt.Errorf("cannot get container name: %w", err)
		}

		defer func() {
			*counter = (*counter) + 1
		}()

		return fmt.Sprintf("%s_%d", role.String(), *counter), nil
	}

	imageName := func(role flow.Role) string {
		return fmt.Sprintf("gcr.io/dl-flow/%s:latest", role.String())
	}

	assign := func(node *FlowNode) (*testingdock.ContainerOpts, *flow.Identity, error) {

		name, err := containerName(node.Role)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot conver to container: %w", err)
		}

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

		return opts, &identity, nil
	}

	add := func(opts *testingdock.ContainerOpts, identity *flow.Identity) (*FlowContainer, error) {

		flowContainer := &FlowContainer{
			Identity: *identity,
			Ports:    make(map[string]string),
		}

		opts.Config.Cmd = []string{
			fmt.Sprintf("--entries=%s", networkIdentities),
			fmt.Sprintf("--nodeid=%s", identity.NodeID.String()),
			fmt.Sprintf("--connections=%d", len(nodes)-1),
			"--loglevel=debug",
		}

		switch identity.Role {

		// enhance with extras for collection node
		case flow.RoleCollection:

			collectionNodeApiPort := testingdock.RandomPort(t)

			opts.Config.ExposedPorts = nat.PortSet{
				"9000/tcp": struct{}{},
			}
			opts.Config.Cmd = append(opts.Config.Cmd, fmt.Sprintf("--ingress-addr=%s:9000", opts.Name))
			opts.HostConfig = &container.HostConfig{
				PortBindings: nat.PortMap{
					nat.Port("9000/tcp"): []nat.PortBinding{
						{
							HostIP:   "0.0.0.0",
							HostPort: collectionNodeApiPort,
						},
					},
				},
			}
			opts.HealthCheck = testingdock.HealthCheckCustom(func() error {
				return healthcheckGRPC(collectionNodeApiPort, context)
			})

			flowContainer.Ports["api"] = collectionNodeApiPort
		}

		c := suite.Container(*opts)

		network.After(c)
		flowContainer.Container = c

		return flowContainer, nil
	}

	for i, node := range nodes {
		ops, identity, err := assign(node)
		if err != nil {
			return nil, fmt.Errorf("cannot conver to container opts: %w", err)
		}
		opts[i] = ops
		identities[i] = identity
		identitiesStr[i] = identity.String()
	}

	networkIdentities = strings.Join(identitiesStr, ",")

	for i := range nodes {

		c, err := add(opts[i], identities[i])
		if err != nil {
			return nil, fmt.Errorf("cannot create container: %w", err)
		}

		containers[i] = c
	}

	return &FlowNetwork{
		Suite:      suite,
		Network:    network,
		Containers: containers,
	}, nil
}
