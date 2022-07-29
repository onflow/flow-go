package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/plus3it/gorecurcopy"

	"github.com/onflow/flow-go/cmd/bootstrap/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

const (
	BootstrapDir             = "./bootstrap"
	ProfilerDir              = "./profiler"
	DataDir                  = "./data"
	TrieDir                  = "./trie"
	DockerComposeFile        = "./docker-compose.nodes.yml"
	DockerComposeFileVersion = "3.7"
	PrometheusTargetsFile    = "./targets.nodes.json"
	DefaultAccessGatewayName = "access_1"
	DefaultObserverName      = "observer"
	DefaultMaxObservers      = 1000
	DefaultCollectionCount   = 3
	DefaultConsensusCount    = 3
	DefaultExecutionCount    = 1
	DefaultVerificationCount = 1
	DefaultAccessCount       = 1
	DefaultObserverCount     = 0
	DefaultNClusters         = 1
	DefaultProfiler          = false
	DefaultTracing           = true
	DefaultCadenceTracing    = false
	DefaultExtensiveTracing  = false
	DefaultConsensusDelay    = 800 * time.Millisecond
	DefaultCollectionDelay   = 950 * time.Millisecond
	AccessAPIPort            = 3569
	AccessPubNetworkPort     = 1234
	ExecutionAPIPort         = 3600
	MetricsPort              = 8080
	RPCPort                  = 9000
	SecuredRPCPort           = 9001
	AdminToolPort            = 9002
	AdminToolLocalPort       = 3700
	HTTPPort                 = 8000
)

var (
	collectionCount        int
	consensusCount         int
	executionCount         int
	verificationCount      int
	accessCount            int
	observerCount          int
	nClusters              uint
	numViewsInStakingPhase uint64
	numViewsInDKGPhase     uint64
	numViewsEpoch          uint64
	profiler               bool
	tracing                bool
	cadenceTracing         bool
	extesiveTracing        bool
	consensusDelay         time.Duration
	collectionDelay        time.Duration
)

func init() {
	flag.IntVar(&collectionCount, "collection", DefaultCollectionCount, "number of collection nodes")
	flag.IntVar(&consensusCount, "consensus", DefaultConsensusCount, "number of consensus nodes")
	flag.IntVar(&executionCount, "execution", DefaultExecutionCount, "number of execution nodes")
	flag.IntVar(&verificationCount, "verification", DefaultVerificationCount, "number of verification nodes")
	flag.IntVar(&accessCount, "access", DefaultAccessCount, "number of staked access nodes")
	flag.IntVar(&observerCount, "observer", DefaultObserverCount, "number of observers")
	flag.UintVar(&nClusters, "nclusters", DefaultNClusters, "number of collector clusters")
	flag.Uint64Var(&numViewsEpoch, "epoch-length", 10000, "number of views in epoch")
	flag.Uint64Var(&numViewsInStakingPhase, "epoch-staking-phase-length", 2000, "number of views in epoch staking phase")
	flag.Uint64Var(&numViewsInDKGPhase, "epoch-dkg-phase-length", 2000, "number of views in epoch dkg phase")
	flag.BoolVar(&profiler, "profiler", DefaultProfiler, "whether to enable the auto-profiler")
	flag.BoolVar(&tracing, "tracing", DefaultTracing, "whether to enable low-overhead tracing in flow")
	flag.BoolVar(&cadenceTracing, "cadence-tracing", DefaultCadenceTracing, "whether to enable the tracing in cadance")
	flag.BoolVar(&extesiveTracing, "extensive-tracing", DefaultExtensiveTracing, "enables high-overhead tracing in fvm")
	flag.DurationVar(&consensusDelay, "consensus-delay", DefaultConsensusDelay, "delay on consensus node block proposals")
	flag.DurationVar(&collectionDelay, "collection-delay", DefaultCollectionDelay, "delay on collection node block proposals")
}

func generateBootstrapData(flowNetworkConf testnet.NetworkConfig) []testnet.ContainerConfig {
	// Prepare localnet host folders, mapped to Docker container volumes upon `docker compose up`
	prepareCommonHostFolders()
	bootstrapData, err := testnet.BootstrapNetwork(flowNetworkConf, BootstrapDir, flow.Localnet)
	if err != nil {
		panic(err)
	}
	fmt.Println("Flow test network bootstrapping data generated...")
	return bootstrapData.StakedConfs
}

// localnet/bootstrap.go generates a docker compose file with images configured for a
// self-contained Flow network, and other peripheral services, such as Observer services.
// Private/Public keys and data are removed and re-created for the self-contained localnet
// environment with every invocation, and are intended solely for testing, not for production.
func main() {
	flag.Parse()

	// Prepare test node configurations of each type, access, execution, verification, etc
	flowNodes := prepareFlowNodes()

	// Generate a Flow network config for localnet
	flowNetworkOpts := []testnet.NetworkConfigOpt{testnet.WithClusters(nClusters)}
	if numViewsEpoch != 0 {
		flowNetworkOpts = append(flowNetworkOpts, testnet.WithViewsInEpoch(numViewsEpoch))
	}
	if numViewsInStakingPhase != 0 {
		flowNetworkOpts = append(flowNetworkOpts, testnet.WithViewsInStakingAuction(numViewsInStakingPhase))
	}
	if numViewsInDKGPhase != 0 {
		flowNetworkOpts = append(flowNetworkOpts, testnet.WithViewsInDKGPhase(numViewsInDKGPhase))
	}
	flowNetworkConf := testnet.NewNetworkConfig("localnet", flowNodes, flowNetworkOpts...)
	displayFlowNetworkConf(flowNetworkConf)

	// Generate the Flow network bootstrap files for this localnet
	flowNodeContainerConfigs := generateBootstrapData(flowNetworkConf)

	// Generate Flow network services docker-compose-nodes.yml
	dockerServices := make(Services)
	dockerServices = prepareFlowServices(dockerServices, flowNodeContainerConfigs)
	serviceDisc := prepareServiceDiscovery(flowNodeContainerConfigs)
	err := writePrometheusConfig(serviceDisc)
	if err != nil {
		panic(err)
	}

	dockerServices = prepareObserverServices(dockerServices, flowNodeContainerConfigs)

	err = writeDockerComposeConfig(dockerServices)
	if err != nil {
		panic(err)
	}

	fmt.Print("Bootstrapping success!\n\n")
	displayPortAssignments()
	fmt.Println()

	fmt.Print("Run \"make start\" to launch the network.\n")
}

func displayFlowNetworkConf(flowNetworkConf testnet.NetworkConfig) {
	fmt.Printf("Network config:\n")
	fmt.Printf("- Clusters: %d\n", flowNetworkConf.NClusters)
	fmt.Printf("- Epoch Length: %d\n", flowNetworkConf.ViewsInEpoch)
	fmt.Printf("- Staking Phase Length: %d\n", flowNetworkConf.ViewsInStakingAuction)
	fmt.Printf("- DKG Phase Length: %d\n", flowNetworkConf.ViewsInDKGPhase)
}

func displayPortAssignments() {
	for i := 0; i < accessCount; i++ {
		fmt.Printf("Access %d Flow API will be accessible at localhost:%d\n", i+1, AccessAPIPort+i)
		fmt.Printf("Access %d public libp2p access will be accessible at localhost:%d\n\n", i+1, AccessPubNetworkPort+i)
	}
	for i := 0; i < executionCount; i++ {
		fmt.Printf("Execution API %d will be accessible at localhost:%d\n", i+1, ExecutionAPIPort+i)
	}
	fmt.Println()
	for i := 0; i < observerCount; i++ {
		fmt.Printf("Observer %d Flow API will be accessible at localhost:%d\n", i+1, (accessCount*2)+(AccessAPIPort)+2*i)
	}
}

func prepareCommonHostFolders() {
	for _, dir := range []string{BootstrapDir, ProfilerDir, DataDir, TrieDir} {
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			panic(err)
		}

		if err := os.Mkdir(dir, 0755); err != nil {
			panic(err)
		}
	}
}

func prepareFlowNodes() []testnet.NodeConfig {
	fmt.Println("Bootstrapping a new self-contained, self-connected Flow network...")
	fmt.Printf("Flow node counts:\n")
	fmt.Printf("- Collection: %d\n", collectionCount)
	fmt.Printf("- Consensus: %d\n", consensusCount)
	fmt.Printf("- Execution: %d\n", executionCount)
	fmt.Printf("- Verification: %d\n", verificationCount)
	fmt.Printf("- Access: %d\n", accessCount)
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
	Labels      map[string]string
}

// Build ...
type Build struct {
	Context    string
	Dockerfile string
	Args       map[string]string
	Target     string
}

func prepareFlowServices(services Services, containers []testnet.ContainerConfig) Services {
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

func prepareServiceDirs(role string, nodeId string) (string, string) {
	// create a data dir for the node
	dataDir := "./" + filepath.Join(DataDir, role)
	if nodeId != "" {
		dataDir = "./" + filepath.Join(DataDir, role, nodeId)
	}

	err := os.MkdirAll(dataDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	// create the profiler dir for the node
	profilerDir := "./" + filepath.Join(ProfilerDir, role)
	if nodeId != "" {
		profilerDir = "./" + filepath.Join(ProfilerDir, role, nodeId)
	}
	err = os.MkdirAll(profilerDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	return dataDir, profilerDir
}

func prepareService(container testnet.ContainerConfig, i int, n int) Service {
	dataDir, profilerDir := prepareServiceDirs(container.Role.String(), container.NodeID.String())

	service := Service{
		Image: fmt.Sprintf("localnet-%s", container.Role),
		Command: []string{
			fmt.Sprintf("--nodeid=%s", container.NodeID),
			"--bootstrapdir=/bootstrap",
			"--datadir=/data/protocol",
			"--secretsdir=/data/secret",
			"--loglevel=DEBUG",
			fmt.Sprintf("--profiler-enabled=%t", profiler),
			fmt.Sprintf("--tracer-enabled=%t", tracing),
			"--profiler-dir=/profiler",
			"--profiler-interval=2m",
		},
		Volumes: []string{
			fmt.Sprintf("%s:/bootstrap:z", BootstrapDir),
			fmt.Sprintf("%s:/profiler:z", profilerDir),
			fmt.Sprintf("%s:/data:z", dataDir),
		},
		Environment: []string{
			// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.12.0/specification/protocol/exporter.md
			"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://tempo:4317",
			"OTEL_EXPORTER_OTLP_TRACES_INSECURE=true",
			"BINSTAT_ENABLE",
			"BINSTAT_LEN_WHAT",
			"BINSTAT_DMP_NAME",
			"BINSTAT_DMP_PATH",
		},
		Labels: map[string]string{
			"com.dapperlabs.role": container.Role.String(),
			"com.dapperlabs.num":  fmt.Sprintf("%03d", i+1),
		},
	}

	service.Command = append(service.Command, fmt.Sprintf("--admin-addr=:%v", AdminToolPort))

	// only specify build config for first service of each role
	if i == 0 {
		service.Build = Build{
			Context:    "../../",
			Dockerfile: "cmd/Dockerfile",
			Args: map[string]string{
				"TARGET":  fmt.Sprintf("./cmd/%s", container.Role.String()),
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
		fmt.Sprintf("--cadence-tracing=%t", cadenceTracing),
		fmt.Sprintf("--extensive-tracing=%t", extesiveTracing),
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

	service.Command = append(service.Command,
		fmt.Sprintf("--rpc-addr=%s:%d", container.ContainerName, RPCPort),
		fmt.Sprintf("--secure-rpc-addr=%s:%d", container.ContainerName, SecuredRPCPort),
		fmt.Sprintf("--http-addr=%s:%d", container.ContainerName, HTTPPort),
		fmt.Sprintf("--collection-ingress-port=%d", RPCPort),
		"--supports-observer=true",
		fmt.Sprintf("--public-network-address=%s:%d", container.ContainerName, AccessPubNetworkPort),
		"--log-tx-time-to-finalized",
		"--log-tx-time-to-executed",
		"--log-tx-time-to-finalized-executed",
	)

	service.Ports = []string{
		fmt.Sprintf("%d:%d", AccessPubNetworkPort+i, AccessPubNetworkPort),
		fmt.Sprintf("%d:%d", AccessAPIPort+2*i, RPCPort),
		fmt.Sprintf("%d:%d", AccessAPIPort+(2*i+1), SecuredRPCPort),
	}

	return service
}

func prepareObserverService(i int, observerName string, agPublicKey string) Service {
	// Observers have a unique naming scheme omitting node id being on the public network
	dataDir, profilerDir := prepareServiceDirs(observerName, "")

	observerService := Service{
		Image: fmt.Sprintf("localnet-%s", DefaultObserverName),
		Command: []string{
			fmt.Sprintf("--bootstrap-node-addresses=%s:%d", DefaultAccessGatewayName, AccessPubNetworkPort),
			fmt.Sprintf("--bootstrap-node-public-keys=%s", agPublicKey),
			fmt.Sprintf("--upstream-node-addresses=%s:%d", DefaultAccessGatewayName, SecuredRPCPort),
			fmt.Sprintf("--upstream-node-public-keys=%s", agPublicKey),
			fmt.Sprintf("--observer-networking-key-path=/bootstrap/private-root-information/%s_key", observerName),
			fmt.Sprintf("--bind=0.0.0.0:0"),
			fmt.Sprintf("--rpc-addr=%s:%d", observerName, RPCPort),
			fmt.Sprintf("--secure-rpc-addr=%s:%d", observerName, SecuredRPCPort),
			fmt.Sprintf("--http-addr=%s:%d", observerName, HTTPPort),
			"--bootstrapdir=/bootstrap",
			"--datadir=/data/protocol",
			"--secretsdir=/data/secret",
			"--loglevel=DEBUG",
			fmt.Sprintf("--profiler-enabled=%t", profiler),
			fmt.Sprintf("--tracer-enabled=%t", tracing),
			"--profiler-dir=/profiler",
			"--profiler-interval=2m",
		},
		Volumes: []string{
			fmt.Sprintf("%s:/bootstrap:z", BootstrapDir),
			fmt.Sprintf("%s:/profiler:z", profilerDir),
			fmt.Sprintf("%s:/data:z", dataDir),
		},
		Environment: []string{
			// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.12.0/specification/protocol/exporter.md
			"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://tempo:4317",
			"OTEL_EXPORTER_OTLP_TRACES_INSECURE=true",
			"BINSTAT_ENABLE",
			"BINSTAT_LEN_WHAT",
			"BINSTAT_DMP_NAME",
			"BINSTAT_DMP_PATH",
		},
	}
	observerService.DependsOn = []string{}
	if i == 0 {
		observerService.Build = Build{
			Context:    "../../",
			Dockerfile: "cmd/Dockerfile",
			Args: map[string]string{
				"TARGET":  "./cmd/observer",
				"VERSION": build.Semver(),
				"COMMIT":  build.Commit(),
				"GOARCH":  runtime.GOARCH,
			},
			Target: "production",
		}
	} else {
		// remaining services of this role must depend on first service
		observerService.DependsOn = append(observerService.DependsOn, fmt.Sprintf("%s_1", DefaultObserverName))
	}
	// observer services rely on the access gateway
	observerService.DependsOn = append(observerService.DependsOn, DefaultAccessGatewayName)
	observerService.Ports = []string{
		// Flow API ports come in pairs, open and secure. While the guest port is always
		// the same from the guest's perspective, the host port numbering accounts for the presence
		// of multiple pairs of listeners on the host to avoid port collisions. Observer listener pairs
		// are numbered just after the Access listeners on the host network by prior convention
		fmt.Sprintf("%d:%d", (accessCount*2)+AccessAPIPort+(2*i), RPCPort),
		fmt.Sprintf("%d:%d", (accessCount*2)+AccessAPIPort+(2*i)+1, SecuredRPCPort),
	}
	return observerService
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

// PrometheusServiceDiscovery is a list of prometheus targets
type PrometheusServiceDiscovery []PrometheusTarget

// PrometheusTargetList defines addresses and labels for a prometheus target
type PrometheusTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func prepareServiceDiscovery(containers []testnet.ContainerConfig) PrometheusServiceDiscovery {
	counters := map[flow.Role]int{}

	sd := PrometheusServiceDiscovery{}
	for _, container := range containers {
		counters[container.Role]++
		pt := PrometheusTarget{
			Targets: []string{net.JoinHostPort(container.ContainerName, strconv.Itoa(MetricsPort))},
			Labels: map[string]string{
				"job":     "flow",
				"role":    container.Role.String(),
				"network": "localnet",
				"num":     fmt.Sprintf("%03d", counters[container.Role]),
			},
		}
		sd = append(sd, pt)
	}

	return sd
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

//////
//
// Observer functions
//

func getAccessGatewayPublicKey(flowNodeContainerConfigs []testnet.ContainerConfig) (string, error) {
	for _, container := range flowNodeContainerConfigs {
		if container.ContainerName == DefaultAccessGatewayName {
			// remove the "0x"..0000 portion of the key
			return container.NetworkPubKey().String()[2:], nil
		}
	}
	return "", fmt.Errorf("Unable to find public key for Access Gateway expected in container '%s'", DefaultAccessGatewayName)
}

func writeObserverPrivateKey(observerName string) {
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
	outputFile := fmt.Sprintf("%s/private-root-information/%s_key", BootstrapDir, observerName)
	err = ioutil.WriteFile(outputFile, output, 0600)
	if err != nil {
		panic(err)
	}
}

func prepareObserverServices(dockerServices Services, flowNodeContainerConfigs []testnet.ContainerConfig) Services {
	if observerCount == 0 {
		return dockerServices
	}

	if accessCount < 1 {
		panic("Observers require at least one Access node to serve as an Access Gateway")
	}

	// Some reasonable maximum is needed to prevent conflicts with the Flow API ports
	if observerCount > DefaultMaxObservers {
		panic(fmt.Sprintf("No more than %d observers are permitted within localnet", DefaultMaxObservers))
	}

	agPublicKey, err := getAccessGatewayPublicKey(flowNodeContainerConfigs)
	if err != nil {
		panic(err)
	}

	for i := 0; i < observerCount; i++ {
		observerName := fmt.Sprintf("%s_%d", DefaultObserverName, i+1)
		observerService := prepareObserverService(i, observerName, agPublicKey)

		// Add a docker container for this named Observer
		dockerServices[observerName] = observerService

		// Generate observer private key (localnet only, not for production)
		writeObserverPrivateKey(observerName)
	}
	fmt.Println()
	fmt.Println("Observer services bootstrapping data generated...")
	fmt.Printf("Access Gateway (%s) public network libp2p key: %s\n\n", DefaultAccessGatewayName, agPublicKey)

	return dockerServices
}
