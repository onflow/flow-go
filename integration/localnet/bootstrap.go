package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/plus3it/gorecurcopy"
	"gopkg.in/yaml.v2"

	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

const (
	BootstrapDir               = "./bootstrap"
	ProfilerDir                = "./profiler"
	DataDir                    = "./data"
	TrieDir                    = "./trie"
	DockerComposeFile          = "./docker-compose.nodes.yml"
	DockerComposeFileVersion   = "3.7"
	PrometheusTargetsFile      = "./targets.nodes.json"
	DefaultCollectionCount     = 3
	DefaultConsensusCount      = 3
	DefaultExecutionCount      = 1
	DefaultVerificationCount   = 1
	DefaultAccessCount         = 1
	DefaultUnstakedAccessCount = 0
	DefaultNClusters           = 1
	DefaultProfiler            = false
	DefaultConsensusDelay      = 800 * time.Millisecond
	DefaultCollectionDelay     = 950 * time.Millisecond
	AccessAPIPort              = 3569
	ExecutionAPIPort           = 3600
	MetricsPort                = 8080
	RPCPort                    = 9000
	SecuredRPCPort             = 9001
	AdminToolPort              = 9002
	AdminToolLocalPort         = 3700
	HTTPPort                   = 8000
)

var (
	collectionCount        int
	consensusCount         int
	executionCount         int
	verificationCount      int
	accessCount            int
	unstakedAccessCount    int
	nClusters              uint
	numViewsInStakingPhase uint64
	numViewsInDKGPhase     uint64
	numViewsEpoch          uint64
	profiler               bool
	consensusDelay         time.Duration
	collectionDelay        time.Duration
)

func init() {
	flag.IntVar(&collectionCount, "collection", DefaultCollectionCount, "number of collection nodes")
	flag.IntVar(&consensusCount, "consensus", DefaultConsensusCount, "number of consensus nodes")
	flag.IntVar(&executionCount, "execution", DefaultExecutionCount, "number of execution nodes")
	flag.IntVar(&verificationCount, "verification", DefaultVerificationCount, "number of verification nodes")
	flag.IntVar(&accessCount, "access", DefaultAccessCount, "number of staked access nodes")
	flag.IntVar(&unstakedAccessCount, "unstaked-access", DefaultUnstakedAccessCount, "number of un-staked access nodes")
	flag.UintVar(&nClusters, "nclusters", DefaultNClusters, "number of collector clusters")
	flag.Uint64Var(&numViewsEpoch, "epoch-length", 10000, "number of views in epoch")
	flag.Uint64Var(&numViewsInStakingPhase, "epoch-staking-phase-length", 2000, "number of views in epoch staking phase")
	flag.Uint64Var(&numViewsInDKGPhase, "epoch-dkg-phase-length", 2000, "number of views in epoch dkg phase")
	flag.BoolVar(&profiler, "profiler", DefaultProfiler, "whether to enable the auto-profiler")
	flag.DurationVar(&consensusDelay, "consensus-delay", DefaultConsensusDelay, "delay on consensus node block proposals")
	flag.DurationVar(&collectionDelay, "collection-delay", DefaultCollectionDelay, "delay on collection node block proposals")
}

func main() {
	flag.Parse()

	fmt.Println("Bootstrapping a new FLITE network...")

	fmt.Printf("Node counts:\n")
	fmt.Printf("- Collection: %d\n", collectionCount)
	fmt.Printf("- Consensus: %d\n", consensusCount)
	fmt.Printf("- Execution: %d\n", executionCount)
	fmt.Printf("- Verification: %d\n", verificationCount)
	fmt.Printf("- Staked Access: %d\n", accessCount)
	fmt.Printf("- Unstaked Access: %d\n\n", unstakedAccessCount)

	nodes := prepareNodes()

	opts := []testnet.NetworkConfigOpt{testnet.WithClusters(nClusters)}
	if numViewsEpoch != 0 {
		opts = append(opts, testnet.WithViewsInEpoch(numViewsEpoch))
	}
	if numViewsInStakingPhase != 0 {
		opts = append(opts, testnet.WithViewsInStakingAuction(numViewsInStakingPhase))
	}
	if numViewsInDKGPhase != 0 {
		opts = append(opts, testnet.WithViewsInDKGPhase(numViewsInDKGPhase))
	}
	conf := testnet.NewNetworkConfig("localnet", nodes, opts...)

	fmt.Printf("Network config:\n")
	fmt.Printf("- Clusters: %d\n", conf.NClusters)
	fmt.Printf("- Epoch Length: %d\n", conf.ViewsInEpoch)
	fmt.Printf("- Staking Phase Length: %d\n", conf.ViewsInStakingAuction)
	fmt.Printf("- DKG Phase Length: %d\n", conf.ViewsInDKGPhase)

	err := os.RemoveAll(BootstrapDir)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	err = os.Mkdir(BootstrapDir, 0755)
	if err != nil {
		panic(err)
	}

	err = os.RemoveAll(ProfilerDir)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	err = os.Mkdir(ProfilerDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	err = os.RemoveAll(DataDir)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	err = os.Mkdir(DataDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	err = os.RemoveAll(TrieDir)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	err = os.Mkdir(TrieDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	_, _, _, containers, _, err := testnet.BootstrapNetwork(conf, BootstrapDir)
	if err != nil {
		panic(err)
	}

	fmt.Println("Node bootstrapping data generated...")

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

	for i := 0; i < accessCount; i++ {
		fmt.Printf("Access API %d will be accessible at localhost:%d\n", i+1, AccessAPIPort+i)
	}
	for i := 0; i < executionCount; i++ {
		fmt.Printf("Execution API %d will be accessible at localhost:%d\n", i+1, ExecutionAPIPort+i)
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

	for i := 0; i < unstakedAccessCount; i++ {
		nodes = append(nodes, testnet.NewNodeConfig(flow.RoleAccess, func(cfg *testnet.NodeConfig) {
			cfg.SupportsUnstakedNodes = true
		}))
	}

	return nodes
}

// Network ...
type Network struct {
	Version  string
	Services Services
}

// Services ...
type Services map[string]Service

// Service ...
type Service struct {
	Build       Build `yaml:"build,omitempty"`
	Image       string
	DependsOn   []string `yaml:"depends_on,omitempty"`
	Command     []string
	Environment []string `yaml:"environment,omitempty"`
	Volumes     []string
	Ports       []string `yaml:"ports,omitempty"`
}

// Build ...
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

	for n, container := range containers {
		switch container.Role {
		case flow.RoleConsensus:
			services[container.ContainerName] = prepareConsensusService(
				container,
				numConsensus,
				n,
			)
			numConsensus++
		case flow.RoleCollection:
			services[container.ContainerName] = prepareCollectionService(
				container,
				numCollection,
				n,
			)
			numCollection++
		case flow.RoleExecution:
			services[container.ContainerName] = prepareExecutionService(container, numExecution, n)
			numExecution++
		case flow.RoleVerification:
			services[container.ContainerName] = prepareVerificationService(container, numVerification, n)
			numVerification++
		case flow.RoleAccess:
			services[container.ContainerName] = prepareAccessService(container, numAccess, n)
			numAccess++
		}
	}

	return services
}

func prepareService(container testnet.ContainerConfig, i int, n int) Service {

	// create a data dir for the node
	dataDir := "./" + filepath.Join(DataDir, container.Role.String(), container.NodeID.String())
	err := os.MkdirAll(dataDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	// create the profiler dir for the node
	profilerDir := "./" + filepath.Join(ProfilerDir, container.Role.String(), container.NodeID.String())
	err = os.MkdirAll(profilerDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	service := Service{
		Image: fmt.Sprintf("localnet-%s", container.Role),
		Command: []string{
			fmt.Sprintf("--nodeid=%s", container.NodeID),
			"--bootstrapdir=/bootstrap",
			"--datadir=/data/protocol",
			"--secretsdir=/data/secret",
			"--loglevel=DEBUG",
			fmt.Sprintf("--profiler-enabled=%t", profiler),
			// TODO change it to flag
			fmt.Sprintf("--tracer-enabled=%t", true),
			"--profiler-dir=/profiler",
			"--profiler-interval=2m",
		},
		Volumes: []string{
			fmt.Sprintf("%s:/bootstrap:z", BootstrapDir),
			fmt.Sprintf("%s:/profiler:z", profilerDir),
			fmt.Sprintf("%s:/data:z", dataDir),
		},
		Environment: []string{
			"JAEGER_AGENT_HOST=jaeger",
			"JAEGER_AGENT_PORT=6831",
			// NOTE: these env vars are not set by default, but can be set [1] to enable binstat logging:
			// [1] https://docs.docker.com/compose/environment-variables/#pass-environment-variables-to-containers
			"BINSTAT_ENABLE",
			"BINSTAT_LEN_WHAT",
			"BINSTAT_DMP_NAME",
			"BINSTAT_DMP_PATH",
		},
	}

	// TODO: having trouble enabling admin tool for access node. skip Access node for now.
	if container.Role != flow.RoleAccess {
		service.Command = append(service.Command, fmt.Sprintf("--admin-addr=:%v", AdminToolPort))
	}

	// only specify build config for first service of each role
	if i == 0 {
		service.Build = Build{
			Context:    "../../",
			Dockerfile: "cmd/Dockerfile",
			Args: map[string]string{
				"TARGET":  container.Role.String(),
				"VERSION": build.Semver(),
				"COMMIT":  build.Commit(),
				"GOARCH":  runtime.GOARCH,
			},
			Target: "production",
		}

		// bring up access node before any other nodes
		if container.Role == flow.RoleConsensus || container.Role == flow.RoleCollection {
			service.DependsOn = []string{"access_1"}
		}

	} else {
		// remaining services of this role must depend on first service
		service.DependsOn = []string{
			fmt.Sprintf("%s_1", container.Role),
		}
	}

	return service
}

// NOTE: accessNodeIDS is a comma separated list of access node IDS
func prepareConsensusService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	timeout := 1200*time.Millisecond + consensusDelay
	service.Command = append(
		service.Command,
		fmt.Sprintf("--block-rate-delay=%s", consensusDelay),
		fmt.Sprintf("--hotstuff-timeout=%s", timeout),
		fmt.Sprintf("--hotstuff-min-timeout=%s", timeout),
		fmt.Sprintf("--chunk-alpha=1"),
		fmt.Sprintf("--emergency-sealing-active=false"),
		fmt.Sprintf("--insecure-access-api=false"),
		fmt.Sprint("--access-node-ids=*"),
	)

	service.Ports = []string{
		fmt.Sprintf("%d:%d", AdminToolLocalPort+n, AdminToolPort),
	}

	return service
}

func prepareVerificationService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	service.Command = append(
		service.Command,
		fmt.Sprintf("--chunk-alpha=1"),
	)

	service.Ports = []string{
		fmt.Sprintf("%d:%d", AdminToolLocalPort+n, AdminToolPort),
	}

	return service
}

// NOTE: accessNodeIDS is a comma separated list of access node IDS
func prepareCollectionService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	timeout := 1200*time.Millisecond + collectionDelay
	service.Command = append(
		service.Command,
		fmt.Sprintf("--block-rate-delay=%s", collectionDelay),
		fmt.Sprintf("--hotstuff-timeout=%s", timeout),
		fmt.Sprintf("--hotstuff-min-timeout=%s", timeout),
		fmt.Sprintf("--ingress-addr=%s:%d", container.ContainerName, RPCPort),
		fmt.Sprintf("--insecure-access-api=false"),
		fmt.Sprint("--access-node-ids=*"),
	)

	service.Ports = []string{
		fmt.Sprintf("%d:%d", AdminToolLocalPort+n, AdminToolPort),
	}

	return service
}

func prepareExecutionService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	// create the execution state dir for the node
	trieDir := "./" + filepath.Join(TrieDir, container.Role.String(), container.NodeID.String())
	err := os.MkdirAll(trieDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	// we need to actually copy the execution state into the directory for bootstrapping
	sourceDir := "./" + filepath.Join(BootstrapDir, bootstrap.DirnameExecutionState)
	err = gorecurcopy.CopyDirectory(sourceDir, trieDir)
	if err != nil {
		panic(err)
	}

	service.Command = append(
		service.Command,
		"--triedir=/trie",
		fmt.Sprintf("--rpc-addr=%s:%d", container.ContainerName, RPCPort),
	)

	service.Volumes = append(
		service.Volumes,
		fmt.Sprintf("%s:/trie:z", trieDir),
	)

	service.Ports = []string{
		fmt.Sprintf("%d:%d", ExecutionAPIPort+2*i, RPCPort),
		fmt.Sprintf("%d:%d", ExecutionAPIPort+(2*i+1), SecuredRPCPort),
		fmt.Sprintf("%d:%d", AdminToolLocalPort+n, AdminToolPort),
	}

	return service
}

func prepareAccessService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	service.Command = append(service.Command, []string{
		fmt.Sprintf("--rpc-addr=%s:%d", container.ContainerName, RPCPort),
		fmt.Sprintf("--secure-rpc-addr=%s:%d", container.ContainerName, SecuredRPCPort),
		fmt.Sprintf("--http-addr=%s:%d", container.ContainerName, HTTPPort),
		fmt.Sprintf("--collection-ingress-port=%d", RPCPort),
		"--log-tx-time-to-finalized",
		"--log-tx-time-to-executed",
		"--log-tx-time-to-finalized-executed",
	}...)

	service.Ports = []string{
		fmt.Sprintf("%d:%d", AccessAPIPort+2*i, RPCPort),
		fmt.Sprintf("%d:%d", AccessAPIPort+(2*i+1), SecuredRPCPort),
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

// PrometheusServiceDiscovery ...
type PrometheusServiceDiscovery []PromtheusTargetList

// PromtheusTargetList ...
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
