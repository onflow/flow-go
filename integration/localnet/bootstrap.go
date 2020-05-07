package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	BootstrapDir             = "./bootstrap"
	DockerComposeFile        = "./docker-compose.nodes.yml"
	DockerComposeFileVersion = "3.7"
	PrometheusTargetsFile    = "./targets.nodes.json"
	DefaultCollectionCount   = 1
	DefaultConsensusCount    = 3
	DefaultExecutionCount    = 1
	DefaultVerificationCount = 1
	DefaultAccessCount       = 1
	AccessAPIPort            = 3569
	MetricsPort              = 8080
	RPCPort                  = 9000
	PprofPort                = 6000
)

var (
	collectionCount   int
	consensusCount    int
	executionCount    int
	verificationCount int
	accessCount       int
)

func init() {
	flag.IntVar(&collectionCount, "collection", DefaultCollectionCount, "number of collection nodes")
	flag.IntVar(&consensusCount, "consensus", DefaultConsensusCount, "number of consensus nodes")
	flag.IntVar(&executionCount, "execution", DefaultExecutionCount, "number of execution nodes")
	flag.IntVar(&verificationCount, "verification", DefaultVerificationCount, "number of verification nodes")
	flag.IntVar(&accessCount, "access", DefaultAccessCount, "number of access nodes")
}

func main() {
	flag.Parse()

	fmt.Println("Bootstrapping a new FLITE network...")

	fmt.Println()
	fmt.Printf("Node counts:\n")
	fmt.Printf("- Collection: %d\n", collectionCount)
	fmt.Printf("- Consensus: %d\n", consensusCount)
	fmt.Printf("- Execution: %d\n", executionCount)
	fmt.Printf("- Verification: %d\n", verificationCount)
	fmt.Printf("- Access: %d\n\n", accessCount)

	nodes := prepareNodes()

	conf := testnet.NewNetworkConfig("localnet", nodes)

	err := os.RemoveAll(BootstrapDir)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
	}

	err = os.Mkdir(BootstrapDir, 0755)
	if err != nil {
		panic(err)
	}

	_, containers, err := testnet.BootstrapNetwork(conf, BootstrapDir)
	if err != nil {
		panic(err)
	}

	services := prepareServices(containers)

	err = writeDockerComposeConfig(services)
	if err != nil {
		panic(err)
	}

	serviceDisc := prepareServiceDiscovery(containers)

	err = writePrometheusConfig(serviceDisc)
	if err != nil {
		panic(err)
	}

	fmt.Print("Bootstrapping success!\n\n")

	fmt.Println("Profilers:")
	for i, c := range containers {
		fmt.Printf("- %s pprof server will be accessible at localhost:%d\n", c.ContainerName, PprofPort+i)
	}
	fmt.Println()

	fmt.Println("Access APIs:")
	for i := 0; i < accessCount; i++ {
		fmt.Printf("- access_%d gRPC API will be accessible at localhost:%d\n", i+1, AccessAPIPort+i)
	}
	fmt.Println()

	fmt.Print("Run \"make start\" to launch the network.\n")
}

func prepareNodes() []testnet.NodeConfig {
	nodes := make([]testnet.NodeConfig, 0)

	for i := 0; i < collectionCount; i++ {
		nodes = append(nodes, testnet.NewNodeConfig(flow.RoleCollection))
	}

	for i := 0; i < consensusCount; i++ {
		nodes = append(nodes, testnet.NewNodeConfig(flow.RoleConsensus))
	}

	for i := 0; i < executionCount; i++ {
		nodes = append(nodes, testnet.NewNodeConfig(flow.RoleExecution))
	}

	for i := 0; i < verificationCount; i++ {
		nodes = append(nodes, testnet.NewNodeConfig(flow.RoleVerification))
	}

	for i := 0; i < accessCount; i++ {
		nodes = append(nodes, testnet.NewNodeConfig(flow.RoleAccess))
	}

	return nodes
}

type Network struct {
	Version  string
	Services Services
}

type Services map[string]Service

type Service struct {
	Build       Build `yaml:"build,omitempty"`
	Image       string
	DependsOn   []string `yaml:"depends_on,omitempty"`
	Command     []string
	Environment []string `yaml:"environment,omitempty"`
	Volumes     []string
	Ports       []string `yaml:"ports,omitempty"`
}

type Build struct {
	Context    string
	Dockerfile string
	Args       map[string]string
	Target     string
}

func prepareServices(containers []testnet.ContainerConfig) Services {
	services := make(Services)

	var (
		numCollection   = 0
		numConsensus    = 0
		numExecution    = 0
		numVerification = 0
		numAccess       = 0
	)

	for i, container := range containers {
		switch container.Role {
		case flow.RoleConsensus:
			services[container.ContainerName] = prepareService(container, i, numConsensus)
			numConsensus++
		case flow.RoleCollection:
			services[container.ContainerName] = prepareCollectionService(container, i, numCollection)
			numCollection++
		case flow.RoleExecution:
			services[container.ContainerName] = prepareExecutionService(container, i, numExecution)
			numExecution++
		case flow.RoleVerification:
			services[container.ContainerName] = prepareService(container, i, numVerification)
			numVerification++
		case flow.RoleAccess:
			services[container.ContainerName] = prepareAccessService(container, i, numAccess)
			numAccess++
		}
	}

	return services
}

func prepareService(container testnet.ContainerConfig, serviceIndex, roleIndex int) Service {
	service := Service{
		Image: fmt.Sprintf("localnet-%s", container.Role),
		Command: []string{
			fmt.Sprintf("--nodeid=%s", container.NodeID),
			"--bootstrapdir=/bootstrap",
			"--datadir=/flowdb",
			"--loglevel=DEBUG",
			"--nclusters=1",
			"--pprofport=6000",
		},
		Volumes: []string{
			fmt.Sprintf("%s:/bootstrap", BootstrapDir),
		},
		Environment: []string{
			"JAEGER_AGENT_HOST=jaeger",
			"JAEGER_AGENT_PORT=6831",
		},
		Ports: []string{
			fmt.Sprintf("%d:%d", PprofPort+serviceIndex, PprofPort),
		},
	}

	// only specify build config for first service of each role
	if roleIndex == 0 {
		service.Build = Build{
			Context:    "../../",
			Dockerfile: "cmd/Dockerfile",
			Args: map[string]string{
				"TARGET": container.Role.String(),
			},
			Target: "production",
		}
	} else {
		// remaining services of this role must depend on first service
		service.DependsOn = []string{
			fmt.Sprintf("%s_1", container.Role),
		}
	}

	return service
}

func prepareCollectionService(container testnet.ContainerConfig, serviceIndex, roleIndex int) Service {
	service := prepareService(container, serviceIndex, roleIndex)

	service.Command = append(
		service.Command,
		fmt.Sprintf("--ingress-addr=%s:%d", container.ContainerName, RPCPort),
	)

	return service
}

func prepareExecutionService(container testnet.ContainerConfig, serviceIndex, roleIndex int) Service {
	service := prepareService(container, serviceIndex, roleIndex)

	service.Command = append(
		service.Command,
		fmt.Sprintf("--rpc-addr=%s:%d", container.ContainerName, RPCPort),
	)

	return service
}

func prepareAccessService(container testnet.ContainerConfig, serviceIndex, roleIndex int) Service {
	service := prepareService(container, serviceIndex, roleIndex)

	service.Command = append(service.Command, []string{
		fmt.Sprintf("--rpc-addr=%s:%d", container.ContainerName, RPCPort),
		fmt.Sprintf("--ingress-addr=collection_1:%d", RPCPort),
		fmt.Sprintf("--script-addr=execution_1:%d", RPCPort),
	}...)

	service.Ports = append(service.Ports, fmt.Sprintf("%d:%d", AccessAPIPort+roleIndex, RPCPort))

	return service
}

func writeDockerComposeConfig(services Services) error {
	f, err := openAndTruncate(DockerComposeFile)
	if err != nil {
		return err
	}

	network := Network{
		Version:  DockerComposeFileVersion,
		Services: services,
	}

	enc := yaml.NewEncoder(f)

	err = enc.Encode(&network)
	if err != nil {
		return err
	}

	return nil
}

type PrometheusServiceDiscovery []PromtheusTargetList

type PromtheusTargetList struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func newPrometheusTargetList(role flow.Role) PromtheusTargetList {
	return PromtheusTargetList{
		Targets: make([]string, 0),
		Labels: map[string]string{
			"job":  "flow",
			"role": role.String(),
		},
	}
}

func prepareServiceDiscovery(containers []testnet.ContainerConfig) PrometheusServiceDiscovery {
	targets := map[flow.Role]PromtheusTargetList{
		flow.RoleCollection:   newPrometheusTargetList(flow.RoleCollection),
		flow.RoleConsensus:    newPrometheusTargetList(flow.RoleConsensus),
		flow.RoleExecution:    newPrometheusTargetList(flow.RoleExecution),
		flow.RoleVerification: newPrometheusTargetList(flow.RoleVerification),
		flow.RoleAccess:       newPrometheusTargetList(flow.RoleAccess),
	}

	for _, container := range containers {
		containerAddr := fmt.Sprintf("%s:%d", container.ContainerName, MetricsPort)
		containerTargets := targets[container.Role]
		containerTargets.Targets = append(containerTargets.Targets, containerAddr)
		targets[container.Role] = containerTargets
	}

	return PrometheusServiceDiscovery{
		targets[flow.RoleCollection],
		targets[flow.RoleConsensus],
		targets[flow.RoleExecution],
		targets[flow.RoleVerification],
		targets[flow.RoleAccess],
	}
}

func writePrometheusConfig(serviceDisc PrometheusServiceDiscovery) error {
	f, err := openAndTruncate(PrometheusTargetsFile)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)

	err = enc.Encode(&serviceDisc)
	if err != nil {
		return err
	}

	return nil
}

func openAndTruncate(filename string) (*os.File, error) {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, err
	}

	// overwrite current file contents
	err = f.Truncate(0)
	if err != nil {
		return nil, err
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	return f, nil
}
