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
	DefaultConsensusCount    = 3
	AccessAPIPort            = 3569
	MetricsPort              = 8080
)

var consensusCount int

func init() {
	flag.IntVar(&consensusCount, "consensus", DefaultConsensusCount, "number of consensus nodes")
}

func main() {
	flag.Parse()

	fmt.Printf("Bootstrapping a network with %d consensus nodes...\n", consensusCount)

	nodes := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleExecution),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleAccess),
	}

	for i := 0; i < consensusCount; i++ {
		nodes = append(nodes, testnet.NewNodeConfig(flow.RoleConsensus))
	}

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

	containers, err := testnet.BootstrapNetwork(conf, BootstrapDir)
	if err != nil {
		panic(err)
	}

	services := prepareServices(containers)

	err = writeDockerComposeConfig(services)
	if err != nil {
		panic(err)
	}

	err = writePrometheusConfig(containers)
	if err != nil {
		panic(err)
	}

	fmt.Print("Bootstrapping success!\n\n")
	fmt.Printf("The Access API will be accessible at localhost:%d\n\n", AccessAPIPort)
	fmt.Print("Run \"make start\" to launch the network.\n")
}

type Network struct {
	Version  string
	Services Services
}

type Services map[string]Service

type Service struct {
	Build struct {
		Context    string
		Dockerfile string
		Args       map[string]string
		Target     string
	}
	Command []string
	Volumes []string
	Ports   []string
}

func prepareServices(containers []testnet.ContainerConfig) Services {
	services := make(Services)

	for _, container := range containers {
		switch container.Role {
		case flow.RoleCollection:
			services[container.ContainerName] = prepareCollectionService(container)
		case flow.RoleExecution:
			services[container.ContainerName] = prepareExecutionService(container)
		case flow.RoleAccess:
			services[container.ContainerName] = prepareAccessService(container)
		default:
			services[container.ContainerName] = prepareService(container)
		}
	}

	return services
}

func prepareService(container testnet.ContainerConfig) Service {
	return Service{
		Build: struct {
			Context    string
			Dockerfile string
			Args       map[string]string
			Target     string
		}{
			Context:    "../../",
			Dockerfile: "cmd/Dockerfile",
			Args: map[string]string{
				"TARGET": container.Role.String(),
			},
			Target: "production",
		},
		Command: []string{
			fmt.Sprintf("--nodeid=%s", container.NodeID),
			"--bootstrapdir=/bootstrap",
			"--datadir=/flowdb",
			"--loglevel=DEBUG",
			"--nclusters=1",
		},
		Volumes: []string{
			fmt.Sprintf("%s:/bootstrap", BootstrapDir),
		},
	}
}

func prepareCollectionService(container testnet.ContainerConfig) Service {
	service := prepareService(container)

	service.Command = append(
		service.Command,
		fmt.Sprintf("--ingress-addr=%s:9000", container.ContainerName),
	)

	return service
}

func prepareExecutionService(container testnet.ContainerConfig) Service {
	service := prepareService(container)

	service.Command = append(
		service.Command,
		fmt.Sprintf("--rpc-addr=%s:9000", container.ContainerName),
	)

	return service
}

func prepareAccessService(container testnet.ContainerConfig) Service {
	service := prepareService(container)

	service.Command = append(service.Command, []string{
		fmt.Sprintf("--rpc-addr=%s:9000", container.ContainerName),
		"--ingress-addr=collection_1:9000",
		"--script-addr=execution_1:9000",
	}...)

	service.Ports = []string{
		fmt.Sprintf("%d:9000", AccessAPIPort),
	}

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

type PrometheusServiceDiscovery []PromtheusTargets

type PromtheusTargets struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func writePrometheusConfig(containers []testnet.ContainerConfig) error {
	f, err := openAndTruncate(PrometheusTargetsFile)
	if err != nil {
		return err
	}

	targets := make([]string, len(containers))
	for i, container := range containers {
		targets[i] = fmt.Sprintf("%s:%d", container.ContainerName, MetricsPort)
	}

	promServiceDisc := PrometheusServiceDiscovery{
		PromtheusTargets{
			Targets: targets,
			Labels: map[string]string{
				"job": "flow",
			},
		},
	}

	enc := json.NewEncoder(f)

	err = enc.Encode(&promServiceDisc)
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
