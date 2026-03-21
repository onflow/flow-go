package main

import (
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/go-yaml/yaml"

	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/ledger/complete/wal"
	bootstrapFilenames "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"

	"github.com/onflow/flow-go/integration/testnet"
)

const (
	BootstrapDir              = "./bootstrap"
	ProfilerDir               = "./profiler"
	DataDir                   = "./data"
	TrieDir                   = "./trie"
	SocketDir                 = "./sockets"
	DockerComposeFile         = "./docker-compose.nodes.yml"
	DockerComposeFileVersion  = "3.7"
	PrometheusTargetsFile     = "./targets.nodes.json"
	PortMapFile               = "./ports.nodes.json"
	DefaultObserverRole       = "observer"
	DefaultLogLevel           = "DEBUG"
	DefaultGOMAXPROCS         = 8
	DefaultMaxObservers       = 100
	DefaultCollectionCount    = 3
	DefaultConsensusCount     = 3
	DefaultExecutionCount     = 1
	DefaultVerificationCount  = 1
	DefaultAccessCount        = 1
	DefaultObserverCount      = 0
	DefaultTestExecutionCount = 0
	DefaultNClusters          = 1
	DefaultProfiler           = false
	DefaultProfileUploader    = false
	DefaultTracing            = true
	DefaultExtensiveTracing   = false
	DefaultConsensusDelay     = 800 * time.Millisecond
	DefaultCollectionDelay    = 950 * time.Millisecond
)

var (
	collectionCount             int
	consensusCount              int
	executionCount              int
	verificationCount           int
	accessCount                 int
	observerCount               int
	testExecutionCount          int
	ledgerExecutionCount        int
	nClusters                   uint
	numViewsInStakingPhase      uint64
	numViewsInDKGPhase          uint64
	numViewsEpoch               uint64
	kvStoreVersion              string
	epochExtensionViewCount     uint64
	numViewsPerSecond           uint64
	finalizationSafetyThreshold uint64
	profiler                    bool
	profileUploader             bool
	tracing                     bool
	extensiveTracing            bool
	consensusDelay              time.Duration
	collectionDelay             time.Duration
	logLevel                    string

	ports *PortAllocator
)

func init() {
	flag.IntVar(&collectionCount, "collection", DefaultCollectionCount, "number of collection nodes")
	flag.IntVar(&consensusCount, "consensus", DefaultConsensusCount, "number of consensus nodes")
	flag.IntVar(&executionCount, "execution", DefaultExecutionCount, "number of execution nodes")
	flag.IntVar(&verificationCount, "verification", DefaultVerificationCount, "number of verification nodes")
	flag.IntVar(&accessCount, "access", DefaultAccessCount, "number of staked access nodes")
	flag.IntVar(&observerCount, "observer", DefaultObserverCount, "number of observers")
	flag.IntVar(&testExecutionCount, "test-execution", DefaultTestExecutionCount, "number of test execution")
	flag.UintVar(&nClusters, "nclusters", DefaultNClusters, "number of collector clusters")
	flag.Uint64Var(&numViewsEpoch, "epoch-length", 10000, "number of views in epoch")
	flag.Uint64Var(&numViewsInStakingPhase, "epoch-staking-phase-length", 2000, "number of views in epoch staking phase")
	flag.Uint64Var(&numViewsInDKGPhase, "epoch-dkg-phase-length", 2000, "number of views in epoch dkg phase")
	flag.StringVar(&kvStoreVersion, "kvstore-version", "default", "protocol state KVStore version to initialize ('default' or an integer equal to a supported protocol version: '0', '1', '2', ...)")
	flag.Uint64Var(&epochExtensionViewCount, "kvstore-epoch-extension-view-count", 0, "length of epoch extension in views, default is 100_000 which is approximately 1 day")
	flag.Uint64Var(&finalizationSafetyThreshold, "kvstore-finalization-safety-threshold", 0, "number of views for safety threshold T (assume: one finalization occurs within T blocks)")
	flag.Uint64Var(&numViewsPerSecond, "target-view-rate", 1, "target number of views per second")
	flag.BoolVar(&profiler, "profiler", DefaultProfiler, "whether to enable the auto-profiler")
	flag.BoolVar(&profileUploader, "profile-uploader", DefaultProfileUploader, "whether to upload profiles to the cloud")
	flag.BoolVar(&tracing, "tracing", DefaultTracing, "whether to enable low-overhead tracing in flow")
	flag.BoolVar(&extensiveTracing, "extensive-tracing", DefaultExtensiveTracing, "enables high-overhead tracing in fvm")
	flag.DurationVar(&consensusDelay, "consensus-delay", DefaultConsensusDelay, "delay on consensus node block proposals")
	flag.DurationVar(&collectionDelay, "collection-delay", DefaultCollectionDelay, "delay on collection node block proposals")
	flag.StringVar(&logLevel, "loglevel", DefaultLogLevel, "log level for all nodes")
	flag.IntVar(&ledgerExecutionCount, "ledger-execution", 0, "number of execution nodes that use remote ledger service (0 = all use local ledger, max = execution count)")
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

	// Allocate blocks of IPs for each node
	ports = NewPortAllocator()

	// Prepare test node configurations of each type, access, execution, verification, etc
	flowNodes := prepareFlowNodes()

	defaultEpochSafetyParams, err := protocol.DefaultEpochSafetyParams(flow.Localnet)
	if err != nil {
		panic(fmt.Sprintf("could not get default epoch commit safety parameters: %s", err))
	}

	// Generate a Flow network config for localnet
	flowNetworkOpts := []testnet.NetworkConfigOpt{testnet.WithClusters(nClusters)}
	if numViewsEpoch != 0 {
		flowNetworkOpts = append(flowNetworkOpts, testnet.WithViewsInEpoch(numViewsEpoch))
	}
	if numViewsPerSecond != 0 {
		flowNetworkOpts = append(flowNetworkOpts, testnet.WithViewsPerSecond(numViewsPerSecond))
	}
	if numViewsInStakingPhase != 0 {
		flowNetworkOpts = append(flowNetworkOpts, testnet.WithViewsInStakingAuction(numViewsInStakingPhase))
	}
	if numViewsInDKGPhase != 0 {
		flowNetworkOpts = append(flowNetworkOpts, testnet.WithViewsInDKGPhase(numViewsInDKGPhase))
	}

	// Set default finalizationSafetyThreshold if not explicitly set
	if finalizationSafetyThreshold == 0 {
		finalizationSafetyThreshold = defaultEpochSafetyParams.FinalizationSafetyThreshold
	}
	// Set default epochExtensionViewCount if not explicitly set
	if epochExtensionViewCount == 0 {
		epochExtensionViewCount = defaultEpochSafetyParams.EpochExtensionViewCount
	}

	kvStoreFactory := func(epochStateID flow.Identifier) (protocol_state.KVStoreAPI, error) {
		if kvStoreVersion != "default" {
			version, err := strconv.ParseUint(kvStoreVersion, 10, 64)
			if err != nil {
				panic(fmt.Sprintf("--kvstore-version must be a supported integer version number: (eg. '0', '1' or '2', etc.) got %s ", kvStoreVersion))
			}
			return kvstore.NewKVStore(version, finalizationSafetyThreshold, epochExtensionViewCount, epochStateID)

		} else {
			return kvstore.NewDefaultKVStore(finalizationSafetyThreshold, epochExtensionViewCount, epochStateID)
		}
	}
	flowNetworkOpts = append(flowNetworkOpts, testnet.WithKVStoreFactory(kvStoreFactory))

	flowNetworkConf := testnet.NewNetworkConfig("localnet", flowNodes, flowNetworkOpts...)
	displayFlowNetworkConf(flowNetworkConf)

	// Validate ledger execution count
	if ledgerExecutionCount < 0 {
		panic(fmt.Sprintf("ledger-execution must be >= 0, got %d", ledgerExecutionCount))
	}
	if ledgerExecutionCount > executionCount {
		panic(fmt.Sprintf("ledger-execution (%d) must not be greater than execution count (%d)", ledgerExecutionCount, executionCount))
	}

	// Generate the Flow network bootstrap files for this localnet
	flowNodeContainerConfigs := generateBootstrapData(flowNetworkConf)

	// Generate Flow network services docker-compose-nodes.yml
	dockerServices := make(Services)
	dockerServices = prepareFlowServices(dockerServices, flowNodeContainerConfigs)
	serviceDisc := prepareServiceDiscovery(flowNodeContainerConfigs)
	err = writePrometheusConfig(serviceDisc)
	if err != nil {
		panic(err)
	}

	// Only create ledger service if at least one execution node uses remote ledger
	if ledgerExecutionCount > 0 {
		dockerServices = prepareLedgerService(dockerServices, flowNodeContainerConfigs)
	}
	dockerServices = prepareObserverServices(dockerServices, flowNodeContainerConfigs)
	dockerServices = prepareTestExecutionService(dockerServices, flowNodeContainerConfigs)

	err = writeDockerComposeConfig(dockerServices)
	if err != nil {
		panic(err)
	}

	if err = ports.WriteMappingConfig(); err != nil {
		panic(err)
	}

	fmt.Print("Bootstrapping success!\n\n")
	ports.Print()
	fmt.Println()

	fmt.Println("Run \"make start\" to re-build images and launch the network.")
	fmt.Println("Run \"make start-cached\" to launch the network without rebuilding images")
}

func displayFlowNetworkConf(flowNetworkConf testnet.NetworkConfig) {
	fmt.Printf("Network config:\n")
	fmt.Printf("- Clusters: %d\n", flowNetworkConf.NClusters)
	fmt.Printf("- Epoch Length: %d\n", flowNetworkConf.ViewsInEpoch)
	fmt.Printf("- Staking Phase Length: %d\n", flowNetworkConf.ViewsInStakingAuction)
	fmt.Printf("- DKG Phase Length: %d\n", flowNetworkConf.ViewsInDKGPhase)
}

func prepareCommonHostFolders() {
	for _, dir := range []string{BootstrapDir, ProfilerDir, DataDir, TrieDir, SocketDir} {
		if err := os.RemoveAll(dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
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

	name string // don't export
}

func (s *Service) AddExposedPorts(containerPorts ...string) {
	for _, port := range containerPorts {
		s.Ports = append(s.Ports, fmt.Sprintf("%s:%s", ports.HostPort(s.name, port), port))
	}
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
	if err != nil && !errors.Is(err, fs.ErrExist) {
		panic(err)
	}

	// create the profiler dir for the node
	profilerDir := "./" + filepath.Join(ProfilerDir, role)
	if nodeId != "" {
		profilerDir = "./" + filepath.Join(ProfilerDir, role, nodeId)
	}
	err = os.MkdirAll(profilerDir, 0755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		panic(err)
	}

	return dataDir, profilerDir
}

func prepareService(container testnet.ContainerConfig, i int, n int) Service {
	dataDir, profilerDir := prepareServiceDirs(container.Role.String(), container.NodeID.String())

	service := defaultService(container.ContainerName, container.Role.String(), dataDir, profilerDir, i)
	service.Command = append(service.Command,
		fmt.Sprintf("--nodeid=%s", container.NodeID),
	)

	if i == 0 {
		// bring up access node before any other nodes
		if container.Role == flow.RoleConsensus || container.Role == flow.RoleCollection {
			service.DependsOn = append(service.DependsOn, "access_1")
		}
	}

	return service
}

// NOTE: accessNodeIDS is a comma separated list of access node IDS
func prepareConsensusService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	timeout := 1200*time.Millisecond + consensusDelay
	service.Command = append(service.Command,
		fmt.Sprintf("--cruise-ctl-fallback-proposal-duration=%s", consensusDelay),
		fmt.Sprintf("--hotstuff-min-timeout=%s", timeout),
		"--cruise-ctl-max-view-duration=2s",
		"--chunk-alpha=1",
		"--emergency-sealing-active=false",
		"--insecure-access-api=false",
		"--access-node-ids=*",
	)

	return service
}

func prepareVerificationService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	service.Command = append(service.Command,
		"--chunk-alpha=1",
		"--scheduled-callbacks-enabled=true",
	)

	return service
}

// NOTE: accessNodeIDS is a comma separated list of access node IDS
func prepareCollectionService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	timeout := 1200*time.Millisecond + collectionDelay
	service.Command = append(service.Command,
		fmt.Sprintf("--hotstuff-min-timeout=%s", timeout),
		fmt.Sprintf("--ingress-addr=%s:%s", container.ContainerName, testnet.GRPCPort),
		"--insecure-access-api=false",
		"--access-node-ids=*",
	)

	return service
}

func prepareExecutionService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	// create the execution state dir for the node
	trieDir := "./" + filepath.Join(TrieDir, container.Role.String(), container.NodeID.String())
	err := os.MkdirAll(trieDir, 0755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		panic(err)
	}

	service.Command = append(service.Command,
		"--triedir=/trie",
		fmt.Sprintf("--rpc-addr=%s:%s", container.ContainerName, testnet.GRPCPort),
		fmt.Sprintf("--extensive-tracing=%t", extensiveTracing),
		"--execution-data-dir=/data/execution-data",
		"--chunk-data-pack-dir=/data/chunk-data-pack",
		"--pruning-config-threshold=20",
		"--pruning-config-sleep-after-iteration=1m",
		"--scheduled-callbacks-enabled=true",
	)

	// Configure ledger service: execution nodes with index < ledgerExecutionCount use remote ledger
	if i < ledgerExecutionCount {
		// This execution node uses remote ledger service via Unix socket; mount shared socket dir (absolute path)
		absSocketDir, err := filepath.Abs(SocketDir)
		if err != nil {
			panic(fmt.Errorf("socket dir absolute path: %w", err))
		}
		service.Volumes = append(service.Volumes,
			fmt.Sprintf("%s:/sockets:z", absSocketDir),
		)
		service.Command = append(service.Command,
			"--ledger-service-addr=unix:///sockets/ledger.sock",
		)
		// Execution node depends on ledger service
		service.DependsOn = append(service.DependsOn, "ledger_service_1")
		// Execution nodes using remote ledger should NOT mount the trie directory
		// because the ledger service manages it
	} else {
		// Execution nodes with index >= ledgerExecutionCount use local ledger by default (no flag needed)
		// These nodes need to mount the trie directory for their local ledger
		service.Volumes = append(service.Volumes,
			fmt.Sprintf("%s:/trie:z", trieDir),
		)
	}

	service.AddExposedPorts(testnet.GRPCPort)

	return service
}

func prepareAccessService(container testnet.ContainerConfig, i int, n int) Service {
	service := prepareService(container, i, n)

	service.Command = append(service.Command,
		fmt.Sprintf("--rpc-addr=%s:%s", container.ContainerName, testnet.GRPCPort),
		fmt.Sprintf("--secure-rpc-addr=%s:%s", container.ContainerName, testnet.GRPCSecurePort),
		fmt.Sprintf("--http-addr=%s:%s", container.ContainerName, testnet.GRPCWebPort),
		fmt.Sprintf("--rest-addr=%s:%s", container.ContainerName, testnet.RESTPort),
		fmt.Sprintf("--state-stream-addr=%s:%s", container.ContainerName, testnet.GRPCPort),
		fmt.Sprintf("--collection-ingress-port=%s", testnet.GRPCPort),
		"--supports-observer=true",
		fmt.Sprintf("--public-network-address=%s:%s", container.ContainerName, testnet.PublicNetworkPort),
		"--log-tx-time-to-finalized",
		"--log-tx-time-to-executed",
		"--log-tx-time-to-finalized-executed",
		"--execution-data-sync-enabled=true",
		"--execution-data-dir=/data/execution-data",
		"--public-network-execution-data-sync-enabled=true",
		"--execution-data-indexing-enabled=true",
		"--execution-state-dir=/data/execution-state",
		"--script-execution-mode=execution-nodes-only",
		"--event-query-mode=execution-nodes-only",
		"--tx-result-query-mode=execution-nodes-only",
		"--scheduled-callbacks-enabled=true",
		"--extended-indexing-enabled=true",
		"--extended-indexing-db-dir=/data/extended-index",
	)

	service.AddExposedPorts(
		testnet.GRPCPort,
		testnet.GRPCSecurePort,
		testnet.GRPCWebPort,
		testnet.RESTPort,
		testnet.PublicNetworkPort,
	)

	return service
}

func prepareObserverService(i int, observerName string, agPublicKey string) Service {
	// Observers have a unique naming scheme omitting node id being on the public network
	dataDir, profilerDir := prepareServiceDirs(observerName, "")

	service := defaultService(observerName, DefaultObserverRole, dataDir, profilerDir, i)
	service.Command = append(service.Command,
		fmt.Sprintf("--observer-mode-bootstrap-node-addresses=%s:%s", testnet.PrimaryAN, testnet.PublicNetworkPort),
		fmt.Sprintf("--observer-mode-bootstrap-node-public-keys=%s", agPublicKey),
		fmt.Sprintf("--upstream-node-addresses=%s:%s", testnet.PrimaryAN, testnet.GRPCSecurePort),
		fmt.Sprintf("--upstream-node-public-keys=%s", agPublicKey),
		fmt.Sprintf("--observer-networking-key-path=/bootstrap/private-root-information/%s_key", observerName),
		"--bind=0.0.0.0:3569",
		fmt.Sprintf("--rpc-addr=%s:%s", observerName, testnet.GRPCPort),
		fmt.Sprintf("--secure-rpc-addr=%s:%s", observerName, testnet.GRPCSecurePort),
		fmt.Sprintf("--http-addr=%s:%s", observerName, testnet.GRPCWebPort),
		fmt.Sprintf("--rest-addr=%s:%s", observerName, testnet.RESTPort),
		fmt.Sprintf("--state-stream-addr=%s:%s", observerName, testnet.GRPCPort),
		"--execution-data-dir=/data/execution-data",
		"--execution-data-sync-enabled=true",
		"--execution-data-indexing-enabled=true",
		"--execution-state-dir=/data/execution-state",
		"--event-query-mode=execution-nodes-only",
	)

	service.AddExposedPorts(
		testnet.GRPCPort,
		testnet.GRPCSecurePort,
		testnet.GRPCWebPort,
		testnet.RESTPort,
	)

	// observer services rely on the access gateway
	service.DependsOn = append(service.DependsOn, testnet.PrimaryAN)

	return service
}

func defaultService(name, role, dataDir, profilerDir string, i int) Service {
	err := ports.AllocatePorts(name, role)
	if err != nil {
		panic(err)
	}

	num := fmt.Sprintf("%03d", i+1)
	service := Service{
		name:  name,
		Image: fmt.Sprintf("localnet-%s", role),
		Command: []string{
			"--bootstrapdir=/bootstrap",
			"--datadir=/data/protocol",
			"--secretsdir=/data/secret",
			fmt.Sprintf("--loglevel=%s", logLevel),
			fmt.Sprintf("--profiler-enabled=%t", profiler),
			fmt.Sprintf("--profile-uploader-enabled=%t", profileUploader),
			fmt.Sprintf("--tracer-enabled=%t", tracing),
			"--profiler-dir=/profiler",
			"--profiler-interval=2m",
			fmt.Sprintf("--admin-addr=0.0.0.0:%s", testnet.AdminPort),
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
			fmt.Sprintf("OTEL_RESOURCE_ATTRIBUTES=network=localnet,role=%s,num=%s", role, num),
			fmt.Sprintf("GOMAXPROCS=%d", DefaultGOMAXPROCS),
		},
		Labels: map[string]string{
			"org.flowfoundation.role": role,
			"org.flowfoundation.num":  num,
		},
	}

	service.AddExposedPorts(testnet.AdminPort)

	if i == 0 {
		// only specify build config for first service of each role
		service.Build = Build{
			Context:    "../../",
			Dockerfile: "cmd/Dockerfile",
			Args: map[string]string{
				"TARGET":  fmt.Sprintf("./cmd/%s", role),
				"VERSION": build.Version(),
				"COMMIT":  build.Commit(),
				"GOARCH":  runtime.GOARCH,
			},
			Target: "production",
		}
	} else {
		// remaining services of this role must depend on first service
		service.DependsOn = []string{
			fmt.Sprintf("%s_1", role),
		}
	}

	return service
}

func writeDockerComposeConfig(services Services) error {
	f, err := openAndTruncate(DockerComposeFile)
	if err != nil {
		return err
	}
	defer f.Close()

	network := Network{
		Version:  DockerComposeFileVersion,
		Services: services,
	}

	enc := yaml.NewEncoder(f)

	err = enc.Encode(&network)
	if err != nil {
		return err
	}

	// add networks section
	_, err = f.WriteString(`
networks:
  default:
    name: localnet_network
    driver: bridge
    attachable: true
`)
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
			Targets: []string{net.JoinHostPort(container.ContainerName, testnet.MetricsPort)},
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
	defer f.Close()

	enc := json.NewEncoder(f)

	err = enc.Encode(&serviceDisc)
	if err != nil {
		return err
	}

	return nil
}

func openAndTruncate(filename string) (*os.File, error) {
	// Check if path exists and is a directory, remove it if so
	if fi, err := os.Stat(filename); err == nil {
		if fi.IsDir() {
			if err := os.RemoveAll(filename); err != nil {
				return nil, fmt.Errorf("failed to remove existing directory %s: %w", filename, err)
			}
		}
	}

	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
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
		if container.ContainerName == testnet.PrimaryAN {
			// remove the "0x"..0000 portion of the key
			return container.NetworkPubKey().String()[2:], nil
		}
	}
	return "", fmt.Errorf("Unable to find public key for Access Gateway expected in container '%s'", testnet.PrimaryAN)
}

func getAccessID(flowNodeContainerConfigs []testnet.ContainerConfig) (string, error) {
	for _, container := range flowNodeContainerConfigs {
		if container.Role == flow.RoleAccess {
			return container.NodeID.String(), nil
		}
	}
	return "", fmt.Errorf("Unable to find Access node")
}

func getExecutionNodeConfig(flowNodeContainerConfigs []testnet.ContainerConfig) (testnet.ContainerConfig, error) {
	for _, container := range flowNodeContainerConfigs {
		if container.Role == flow.RoleExecution {
			return container, nil
		}
	}
	return testnet.ContainerConfig{}, fmt.Errorf("Unable to find execution node")
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
		observerName := fmt.Sprintf("%s_%d", DefaultObserverRole, i+1)
		observerService := prepareObserverService(i, observerName, agPublicKey)

		// Add a docker container for this named Observer
		dockerServices[observerName] = observerService

		// Generate observer private key (localnet only, not for production)
		_, err := testnet.WriteObserverPrivateKey(observerName, BootstrapDir)
		if err != nil {
			panic(err)
		}
	}

	fmt.Println()
	fmt.Println("Observer services bootstrapping data generated...")
	fmt.Printf("Access Gateway (%s) public network libp2p key: %s\n\n", testnet.PrimaryAN, agPublicKey)

	return dockerServices
}

func prepareLedgerService(dockerServices Services, flowNodeContainerConfigs []testnet.ContainerConfig) Services {
	// Find the first execution node that uses remote ledger (index 0)
	// The ledger service will reuse its trie directory
	var firstExecutionNode *testnet.ContainerConfig
	executionIndex := 0
	for _, container := range flowNodeContainerConfigs {
		if container.Role == flow.RoleExecution {
			if executionIndex < ledgerExecutionCount {
				firstExecutionNode = &container
				break
			}
			executionIndex++
		}
	}

	if firstExecutionNode == nil {
		panic("failed to find first execution node for ledger service")
	}

	// Use the same trie directory as the first execution node
	trieDir := "./" + filepath.Join(TrieDir, firstExecutionNode.Role.String(), firstExecutionNode.NodeID.String())

	// Ensure trie directory exists for the ledger service
	err := os.MkdirAll(trieDir, 0755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		panic(err)
	}

	// Copy root checkpoint from bootstrap directory to trie directory on the host
	// The symlinks will work inside containers because:
	// 1. Execution node has both /bootstrap and /trie mounted
	// 2. Ledger service has /trie mounted and can follow symlinks to /bootstrap (via execution node's mount)
	// 3. We create symlinks using relative paths that work in both host and container contexts
	bootstrapExecutionStateDir := filepath.Join(BootstrapDir, bootstrapFilenames.DirnameExecutionState)
	checkpointSource := filepath.Join(bootstrapExecutionStateDir, bootstrapFilenames.FilenameWALRootCheckpoint)
	if _, err := os.Stat(checkpointSource); err == nil {
		// Checkpoint exists, create symlinks on host
		// The symlinks will use relative paths that resolve correctly inside containers
		// because both /bootstrap and /trie are mounted in the containers
		_, err = wal.SoftlinkCheckpointFile(bootstrapFilenames.FilenameWALRootCheckpoint, bootstrapExecutionStateDir, trieDir)
		if err != nil {
			panic(fmt.Errorf("failed to create checkpoint symlinks: %w", err))
		}
		fmt.Printf("created checkpoint symlinks in trie directory: %s\n", trieDir)
	} else {
		// Checkpoint doesn't exist, this is expected for fresh bootstrap
		// The execution node will create it when it initializes
		fmt.Printf("root checkpoint not found in %s, ledger service will start with empty state\n", checkpointSource)
	}

	// Allocate ports for ledger service
	ledgerServiceName := "ledger_service_1"
	err = ports.AllocatePorts(ledgerServiceName, "ledger")
	if err != nil {
		panic(err)
	}

	// Shared socket directory: use absolute path so Docker mounts the same host dir in all containers
	absSocketDir, err := filepath.Abs(SocketDir)
	if err != nil {
		panic(fmt.Errorf("socket dir absolute path: %w", err))
	}

	// Create ledger service
	// Use Unix domain socket; ledger and execution nodes share absSocketDir mounted at /sockets
	service := Service{
		name:  ledgerServiceName,
		Image: "localnet-ledger",
		Command: []string{
			"--triedir=/trie",
			"--ledger-service-socket=/sockets/ledger.sock",
			"--mtrie-cache-size=100",
			"--checkpoint-distance=100",
			"--checkpoints-to-keep=3",
			fmt.Sprintf("--loglevel=%s", logLevel),
		},
		Volumes: []string{
			fmt.Sprintf("%s:/trie:z", trieDir),
			fmt.Sprintf("%s:/bootstrap:z", BootstrapDir),
			fmt.Sprintf("%s:/sockets:z", absSocketDir),
		},
		Environment: []string{
			fmt.Sprintf("GOMAXPROCS=%d", DefaultGOMAXPROCS),
		},
		Labels: map[string]string{
			"org.flowfoundation.role": "ledger",
			"org.flowfoundation.num":  "001",
		},
	}

	// Build configuration for ledger service
	service.Build = Build{
		Context:    "../../",
		Dockerfile: "cmd/Dockerfile",
		Args: map[string]string{
			"TARGET":  "./cmd/ledger",
			"VERSION": build.Version(),
			"COMMIT":  build.Commit(),
			"GOARCH":  runtime.GOARCH,
		},
		Target: "production",
	}

	dockerServices[ledgerServiceName] = service

	fmt.Println()
	fmt.Println("Ledger service bootstrapping data generated...")

	return dockerServices
}

func prepareTestExecutionService(dockerServices Services, flowNodeContainerConfigs []testnet.ContainerConfig) Services {
	if testExecutionCount == 0 {
		return dockerServices
	}

	agPublicKey, err := getAccessGatewayPublicKey(flowNodeContainerConfigs)
	if err != nil {
		panic(err)
	}

	publicAccessID, err := getAccessID(flowNodeContainerConfigs)
	if err != nil {
		panic(err)
	}

	containerConfig, err := getExecutionNodeConfig(flowNodeContainerConfigs)
	if err != nil {
		panic(err)
	}

	var nodeid flow.Identifier
	_, _ = crand.Read(nodeid[:])
	address := "test_execution_1:2137"

	observerName := fmt.Sprintf("%s_%d", "test_execution", 1)
	// Generate observer private key (localnet only, not for production)
	nodeinfo, err := testnet.WriteTestExecutionService(nodeid, address, observerName, BootstrapDir)
	if err != nil {
		panic(err)
	}

	containerConfig.NodeInfo = nodeinfo
	containerConfig.ContainerName = observerName
	fmt.Println("NodeID: ", containerConfig)

	observerService := prepareExecutionService(containerConfig, 1, 1)
	observerService.Command = append(observerService.Command,
		"--observer-mode=true",
		fmt.Sprintf("--observer-mode-bootstrap-node-addresses=%s:%s", testnet.PrimaryAN, testnet.PublicNetworkPort),
		fmt.Sprintf("--observer-mode-bootstrap-node-public-keys=%s", agPublicKey),
		fmt.Sprintf("--public-access-id=%s", publicAccessID),
	)

	// Add a docker container for this named Observer
	dockerServices[observerName] = observerService

	fmt.Println()
	fmt.Println("Test execution services bootstrapping data generated...")
	fmt.Printf("Access Gateway (%s) public network libp2p key: %s\n\n", testnet.PrimaryAN, agPublicKey)

	return dockerServices
}
