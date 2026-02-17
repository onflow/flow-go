package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/engine/common/provider"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	exeprovider "github.com/onflow/flow-go/engine/execution/provider"
	exepruner "github.com/onflow/flow-go/engine/execution/pruner"
	"github.com/onflow/flow-go/engine/execution/rpc"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage/store"
)

// ExecutionConfig contains the configs for starting up execution nodes
type ExecutionConfig struct {
	rpcConf                               rpc.Config
	triedir                               string
	executionDataDir                      string
	registerDir                           string
	mTrieCacheSize                        uint32
	transactionResultsCacheSize           uint
	checkpointDistance                    uint
	checkpointsToKeep                     uint
	chunkDataPackDir                      string
	chunkDataPackCheckpointsDir           string
	chunkDataPackCacheSize                uint
	chunkDataPackRequestsCacheSize        uint32
	requestInterval                       time.Duration
	extensiveLog                          bool
	pauseExecution                        bool
	chunkDataPackQueryTimeout             time.Duration
	chunkDataPackDeliveryTimeout          time.Duration
	enableBlockDataUpload                 bool
	gcpBucketName                         string
	s3BucketName                          string
	apiRatelimits                         map[string]int
	apiBurstlimits                        map[string]int
	executionDataAllowedPeers             string
	executionDataPrunerHeightRangeTarget  uint64
	executionDataPrunerThreshold          uint64
	blobstoreRateLimit                    int
	blobstoreBurstLimit                   int
	chunkDataPackRequestWorkers           uint
	maxGracefulStopDuration               time.Duration
	importCheckpointWorkerCount           int
	transactionExecutionMetricsEnabled    bool
	transactionExecutionMetricsBufferSize uint
	scheduleCallbacksEnabled              bool

	computationConfig        computation.ComputationConfig
	receiptRequestWorkers    uint   // common provider engine workers
	receiptRequestsCacheSize uint32 // common provider engine cache size

	// This is included to temporarily work around an issue observed on a small number of ENs.
	// It works around an issue where some collection nodes are not configured with enough
	// this works around an issue where some collection nodes are not configured with enough
	// file descriptors causing connection failures.
	onflowOnlyLNs                      bool
	enableStorehouse                   bool
	enableBackgroundStorehouseIndexing bool
	backgroundIndexerHeightsPerSecond  uint64
	enableChecker                      bool
	publicAccessID                     string

	pruningConfigThreshold           uint64
	pruningConfigBatchSize           uint
	pruningConfigSleepAfterCommit    time.Duration
	pruningConfigSleepAfterIteration time.Duration

	ledgerServiceAddr      string // gRPC address for remote ledger service (empty means use local ledger)
	ledgerServiceAdminAddr string // Admin HTTP address for remote ledger service (for trigger-checkpoint command)
	ledgerMaxRequestSize   uint   // Maximum request message size in bytes for remote ledger client (0 = default 1 GiB)
	ledgerMaxResponseSize  uint   // Maximum response message size in bytes for remote ledger client (0 = default 1 GiB)
}

func (exeConf *ExecutionConfig) SetupFlags(flags *pflag.FlagSet) {
	datadir := "/data"

	flags.StringVarP(&exeConf.rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
	flags.UintVar(&exeConf.rpcConf.DeprecatedMaxMsgSize, "rpc-max-message-size", 0,
		"[deprecated] the maximum message size in bytes for messages sent or received over grpc")
	flags.UintVar(&exeConf.rpcConf.MaxRequestMsgSize, "rpc-max-request-message-size", commonrpc.DefaultExecutionMaxRequestSize,
		"the maximum request message size in bytes for request messages received over grpc by the server")
	flags.UintVar(&exeConf.rpcConf.MaxResponseMsgSize, "rpc-max-response-message-size", commonrpc.DefaultExecutionMaxResponseSize,
		"the maximum message size in bytes for response messages sent over grpc by the server")
	flags.BoolVar(&exeConf.rpcConf.RpcMetricsEnabled, "rpc-metrics-enabled", false, "whether to enable the rpc metrics")
	flags.StringVar(&exeConf.triedir, "triedir", filepath.Join(datadir, "trie"), "directory to store the execution State")
	flags.StringVar(&exeConf.executionDataDir, "execution-data-dir", filepath.Join(datadir, "execution_data"), "directory to use for storing Execution Data")
	flags.StringVar(&exeConf.registerDir, "register-dir", filepath.Join(datadir, "register"), "directory to use for storing registers Data")
	flags.Uint32Var(&exeConf.mTrieCacheSize, "mtrie-cache-size", ledger.DefaultMTrieCacheSize, "cache size for MTrie")
	flags.UintVar(&exeConf.checkpointDistance, "checkpoint-distance", ledger.DefaultCheckpointDistance, "number of WAL segments between checkpoints")
	flags.UintVar(&exeConf.checkpointsToKeep, "checkpoints-to-keep", ledger.DefaultCheckpointsToKeep, "number of recent checkpoints to keep (0 to keep all)")
	flags.UintVar(&exeConf.computationConfig.DerivedDataCacheSize, "cadence-execution-cache", derived.DefaultDerivedDataCacheSize,
		"cache size for Cadence execution")
	flags.BoolVar(&exeConf.computationConfig.ExtensiveTracing, "extensive-tracing", false, "adds high-overhead tracing to execution")
	flags.IntVar(&exeConf.computationConfig.MaxConcurrency, "computer-max-concurrency", 1, "set to greater than 1 to enable concurrent transaction execution")
	flags.StringVar(&exeConf.chunkDataPackDir, "chunk-data-pack-dir", filepath.Join(datadir, "chunk_data_packs"), "directory to use for storing chunk data packs")
	flags.StringVar(&exeConf.chunkDataPackCheckpointsDir, "chunk-data-pack-checkpoints-dir", filepath.Join(datadir, "chunk_data_packs_checkpoints_dir"), "directory to use storing chunk data packs pebble database checkpoints for querying while the node is running")
	flags.UintVar(&exeConf.chunkDataPackCacheSize, "chdp-cache", store.DefaultCacheSize, "cache size for chunk data packs")
	flags.Uint32Var(&exeConf.chunkDataPackRequestsCacheSize, "chdp-request-queue", mempool.DefaultChunkDataPackRequestQueueSize, "queue size for chunk data pack requests")
	flags.DurationVar(&exeConf.requestInterval, "request-interval", 60*time.Second, "the interval between requests for the requester engine")
	flags.Uint32Var(&exeConf.receiptRequestsCacheSize, "receipt-request-cache", provider.DefaultEntityRequestCacheSize, "queue size for entity requests at common provider engine")
	flags.UintVar(&exeConf.receiptRequestWorkers, "receipt-request-workers", provider.DefaultRequestProviderWorkers, "number of workers for entity requests at common provider engine")
	flags.DurationVar(&exeConf.computationConfig.QueryConfig.LogTimeThreshold, "script-log-threshold", query.DefaultLogTimeThreshold,
		"threshold for logging script execution")
	flags.DurationVar(&exeConf.computationConfig.QueryConfig.ExecutionTimeLimit, "script-execution-time-limit", query.DefaultExecutionTimeLimit,
		"script execution time limit")
	flags.Uint64Var(&exeConf.computationConfig.QueryConfig.ComputationLimit, "script-execution-computation-limit", fvm.DefaultComputationLimit,
		"script execution computation limit")
	flags.UintVar(&exeConf.transactionResultsCacheSize, "transaction-results-cache-size", 10000, "number of transaction results to be cached")
	flags.BoolVar(&exeConf.extensiveLog, "extensive-logging", false, "extensive logging logs tx contents and block headers")
	flags.DurationVar(&exeConf.chunkDataPackQueryTimeout, "chunk-data-pack-query-timeout", exeprovider.DefaultChunkDataPackQueryTimeout, "timeout duration to determine a chunk data pack query being slow")
	flags.DurationVar(&exeConf.chunkDataPackDeliveryTimeout, "chunk-data-pack-delivery-timeout", exeprovider.DefaultChunkDataPackDeliveryTimeout, "timeout duration to determine a chunk data pack response delivery being slow")
	flags.UintVar(&exeConf.chunkDataPackRequestWorkers, "chunk-data-pack-workers", exeprovider.DefaultChunkDataPackRequestWorker, "number of workers to process chunk data pack requests")
	flags.BoolVar(&exeConf.pauseExecution, "pause-execution", false, "pause the execution. when set to true, no block will be executed, "+
		"but still be able to serve queries")
	flags.BoolVar(&exeConf.enableBlockDataUpload, "enable-blockdata-upload", false, "enable uploading block data to Cloud Bucket")
	flags.StringVar(&exeConf.gcpBucketName, "gcp-bucket-name", "", "GCP Bucket name for block data uploader")
	flags.StringVar(&exeConf.s3BucketName, "s3-bucket-name", "", "S3 Bucket name for block data uploader")
	flags.StringVar(&exeConf.executionDataAllowedPeers, "execution-data-allowed-requesters", "", "comma separated list of Access node IDs that are allowed to request Execution Data. an empty list allows all peers")
	flags.Uint64Var(&exeConf.executionDataPrunerHeightRangeTarget, "execution-data-height-range-target", 0, "target height range size used to limit the amount of Execution Data kept on disk")
	flags.Uint64Var(&exeConf.executionDataPrunerThreshold, "execution-data-height-range-threshold", 100_000, "height threshold used to trigger Execution Data pruning")
	flags.StringToIntVar(&exeConf.apiRatelimits, "api-rate-limits", map[string]int{}, "per second rate limits for GRPC API methods e.g. Ping=300,ExecuteScriptAtBlockID=500 etc. note limits apply globally to all clients.")
	flags.StringToIntVar(&exeConf.apiBurstlimits, "api-burst-limits", map[string]int{}, "burst limits for gRPC API methods e.g. Ping=100,ExecuteScriptAtBlockID=100 etc. note limits apply globally to all clients.")
	flags.IntVar(&exeConf.blobstoreRateLimit, "blobstore-rate-limit", 0, "per second outgoing rate limit for Execution Data blobstore")
	flags.IntVar(&exeConf.blobstoreBurstLimit, "blobstore-burst-limit", 0, "outgoing burst limit for Execution Data blobstore")
	flags.DurationVar(&exeConf.maxGracefulStopDuration, "max-graceful-stop-duration", stop.DefaultMaxGracefulStopDuration, "the maximum amount of time stop control will wait for ingestion engine to gracefully shutdown before crashing")
	flags.IntVar(&exeConf.importCheckpointWorkerCount, "import-checkpoint-worker-count", 10, "number of workers to import checkpoint file during bootstrap")
	flags.BoolVar(&exeConf.transactionExecutionMetricsEnabled, "tx-execution-metrics", true, "enable collection of transaction execution metrics")
	flags.UintVar(&exeConf.transactionExecutionMetricsBufferSize, "tx-execution-metrics-buffer-size", 200, "buffer size for transaction execution metrics. The buffer size is the number of blocks that are kept in memory by the metrics provider engine")

	var exeConfExecutionDataDBMode string
	flags.StringVar(&exeConfExecutionDataDBMode,
		"execution-data-db",
		execution_data.ExecutionDataDBModePebble.String(),
		"[deprecated] the DB type for execution datastore. it's been deprecated")

	flags.BoolVar(&exeConf.onflowOnlyLNs, "temp-onflow-only-lns", false, "do not use unless required. forces node to only request collections from onflow collection nodes")
	flags.BoolVar(&exeConf.enableStorehouse, "enable-storehouse", false, "enable storehouse to store registers on disk, default is false")
	flags.BoolVar(&exeConf.enableBackgroundStorehouseIndexing, "enable-background-storehouse-indexing", false, "enable background indexing of storehouse data while storehouse is disabled to eliminate downtime when enabling it. default: false.")
	flags.Uint64Var(&exeConf.backgroundIndexerHeightsPerSecond, "background-indexer-heights-per-second", storehouse.DefaultHeightsPerSecond, fmt.Sprintf("rate limit for background indexer in heights per second. 0 means no rate limiting. default: %v", storehouse.DefaultHeightsPerSecond))
	flags.BoolVar(&exeConf.enableChecker, "enable-checker", true, "enable checker to check the correctness of the execution result, default is true")
	flags.BoolVar(&exeConf.scheduleCallbacksEnabled, "scheduled-callbacks-enabled", fvm.DefaultScheduledTransactionsEnabled, "[deprecated] enable execution of scheduled transactions")
	// deprecated. Retain it to prevent nodes that previously had this configuration from crashing.
	var deprecatedEnableNewIngestionEngine bool
	flags.BoolVar(&deprecatedEnableNewIngestionEngine, "enable-new-ingestion-engine", true, "enable new ingestion engine, default is true")
	flags.StringVar(&exeConf.publicAccessID, "public-access-id", "", "public access ID for the node")

	flags.Uint64Var(&exeConf.pruningConfigThreshold, "pruning-config-threshold", exepruner.DefaultConfig.Threshold, "the number of blocks that we want to keep in the database, default 30 days")
	flags.UintVar(&exeConf.pruningConfigBatchSize, "pruning-config-batch-size", exepruner.DefaultConfig.BatchSize, "the batch size is the number of blocks that we want to delete in one batch, default 1200")
	flags.DurationVar(&exeConf.pruningConfigSleepAfterCommit, "pruning-config-sleep-after-commit", exepruner.DefaultConfig.SleepAfterEachBatchCommit, "sleep time after each batch commit, default 1s")
	flags.DurationVar(&exeConf.pruningConfigSleepAfterIteration, "pruning-config-sleep-after-iteration", exepruner.DefaultConfig.SleepAfterEachIteration, "sleep time after each iteration, default max int64")
	flags.StringVar(&exeConf.ledgerServiceAddr, "ledger-service-addr", "", "gRPC address for remote ledger service (TCP: e.g., localhost:9000, or Unix socket: unix:///path/to/socket). If empty, uses local ledger")
	flags.StringVar(&exeConf.ledgerServiceAdminAddr, "ledger-service-admin-addr", "", "admin HTTP address for remote ledger service (e.g., localhost:9003). Used to provide helpful error messages when trigger-checkpoint is called in remote mode")
	flags.UintVar(&exeConf.ledgerMaxRequestSize, "ledger-max-request-size", 0, "maximum request message size in bytes for remote ledger client (0 = default 1 GiB)")
	flags.UintVar(&exeConf.ledgerMaxResponseSize, "ledger-max-response-size", 0, "maximum response message size in bytes for remote ledger client (0 = default 1 GiB)")
}

func (exeConf *ExecutionConfig) ValidateFlags() error {
	if exeConf.enableBlockDataUpload {
		if exeConf.gcpBucketName == "" && exeConf.s3BucketName == "" {
			return fmt.Errorf("invalid flag. gcp-bucket-name or s3-bucket-name required when blockdata-uploader is enabled")
		}
	}
	if exeConf.executionDataAllowedPeers != "" {
		ids := strings.Split(exeConf.executionDataAllowedPeers, ",")
		for _, id := range ids {
			if _, err := flow.HexStringToIdentifier(id); err != nil {
				return fmt.Errorf("invalid node ID in execution-data-allowed-requesters %s: %w", id, err)
			}
		}
	}
	if exeConf.rpcConf.MaxRequestMsgSize == 0 {
		return errors.New("rpc-max-request-message-size must be greater than 0")
	}
	if exeConf.rpcConf.MaxResponseMsgSize == 0 {
		return errors.New("rpc-max-response-message-size must be greater than 0")
	}
	// Explicitly turn off background storehouse indexing when storehouse is enabled
	if exeConf.enableStorehouse {
		exeConf.enableBackgroundStorehouseIndexing = false
	}
	return nil
}
