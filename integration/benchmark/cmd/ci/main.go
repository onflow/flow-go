package main

import (
	"context"
	"flag"
	"net"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
	client "github.com/onflow/flow-go-sdk/access/grpc"

	"github.com/onflow/flow-go/integration/benchmark"
	pb "github.com/onflow/flow-go/integration/benchmark/proto"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

type BenchmarkInfo struct {
	BenchmarkType string
}

// Hardcoded CI values
const (
	loadType                    = "token-transfer"
	metricport                  = uint(8080)
	accessNodeAddress           = "127.0.0.1:3569"
	pushgateway                 = "127.0.0.1:9091"
	accountMultiplier           = 50
	feedbackEnabled             = true
	serviceAccountPrivateKeyHex = unittest.ServiceAccountPrivateKeyHex

	// Auto TPS scaling constants
	additiveIncrease       = 100
	multiplicativeDecrease = 0.9
	adjustInterval         = 20 * time.Second

	// gRPC constants
	defaultMaxMsgSize  = 1024 * 1024 * 16 // 16 MB
	defaultGRPCAddress = "127.0.0.1:4777"
)

func main() {
	logLvl := flag.String("log-level", "info", "set log level")

	// CI relevant flags
	grpcAddressFlag := flag.String("grpc-address", defaultGRPCAddress, "listen address for gRPC server")
	initialTPSFlag := flag.Int("tps-initial", 10, "starting transactions per second")
	maxTPSFlag := flag.Int("tps-max", *initialTPSFlag, "maximum transactions per second allowed")
	minTPSFlag := flag.Int("tps-min", *initialTPSFlag, "minimum transactions per second allowed")
	durationFlag := flag.Duration("duration", 10*time.Minute, "test duration")
	gitRepoPathFlag := flag.String("git-repo-path", "..", "git repo path of the filesystem")
	gitRepoURLFlag := flag.String("git-repo-url", "https://github.com/onflow/flow-go.git", "git repo URL")
	bigQueryProjectFlag := flag.String("bigquery-project", "dapperlabs-data", "project name for the bigquery uploader")
	bigQueryDatasetFlag := flag.String("bigquery-dataset", "dev_src_flow_tps_metrics", "dataset name for the bigquery uploader")
	bigQueryRawTableFlag := flag.String("bigquery-raw-table", "rawResults", "table name for the bigquery raw results")
	flag.Parse()

	chainID := flowsdk.Emulator

	// parse log level and apply to logger
	log := zerolog.New(os.Stderr).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
	lvl, err := zerolog.ParseLevel(strings.ToLower(*logLvl))
	if err != nil {
		log.Fatal().Err(err).Str("strLevel", *logLvl).Msg("invalid log level")
	}
	log = log.Level(lvl)

	git, err := NewGit(log, *gitRepoPathFlag, *gitRepoURLFlag)
	if err != nil {
		log.Fatal().Err(err).Str("path", *gitRepoPathFlag).Msg("failed to clone/open git repo")
	}
	repoInfo, err := git.GetRepoInfo()
	if err != nil {
		log.Fatal().Err(err).Str("path", *gitRepoPathFlag).Msg("failed to get repo info")
	}
	log.Info().Interface("repoInfo", repoInfo).Msg("parsed repo info")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := metrics.NewServer(log, metricport)
	<-server.Ready()
	loaderMetrics := metrics.NewLoaderCollector()

	grpcServerOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(defaultMaxMsgSize),
		grpc.MaxSendMsgSize(defaultMaxMsgSize),
	}
	grpcServer := grpc.NewServer(grpcServerOptions...)
	defer grpcServer.Stop()

	pb.RegisterBenchmarkServer(grpcServer, &benchmarkServer{})

	grpcListener, err := net.Listen("tcp", *grpcAddressFlag)
	if err != nil {
		log.Fatal().Err(err).Str("address", *grpcAddressFlag).Msg("failed to listen")
	}

	go func() {
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatal().Err(err).Msg("failed to serve")
		}
	}()

	sp := benchmark.NewStatsPusher(ctx, log, pushgateway, "loader", prometheus.DefaultGatherer)
	defer sp.Stop()

	addressGen := flowsdk.NewAddressGenerator(chainID)
	serviceAccountAddress := addressGen.NextAddress()
	fungibleTokenAddress := addressGen.NextAddress()
	flowTokenAddress := addressGen.NextAddress()
	log.Info().
		Stringer("serviceAccountAddress", serviceAccountAddress).
		Stringer("fungibleTokenAddress", fungibleTokenAddress).
		Stringer("flowTokenAddress", flowTokenAddress).
		Msg("addresses")

	flowClient, err := client.NewClient(accessNodeAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to initialize Flow client")
	}

	// create context for a single benchmark run
	bCtx, bCancel := context.WithCancel(ctx)

	// prepare load generator
	log.Info().
		Str("load_type", loadType).
		Int("initialTPS", *initialTPSFlag).
		Int("minTPS", *minTPSFlag).
		Int("maxTPS", *maxTPSFlag).
		Dur("duration", *durationFlag).
		Msg("Running load case")

	maxInflight := *maxTPSFlag * accountMultiplier

	workerStatsTracker := benchmark.NewWorkerStatsTracker(bCtx)
	defer workerStatsTracker.Stop()

	statsLogger := benchmark.NewPeriodicStatsLogger(workerStatsTracker, log)
	statsLogger.Start()
	defer statsLogger.Stop()

	lg, err := benchmark.New(
		bCtx,
		log,
		workerStatsTracker,
		loaderMetrics,
		[]access.Client{flowClient},
		benchmark.NetworkParams{
			ServAccPrivKeyHex:     serviceAccountPrivateKeyHex,
			ServiceAccountAddress: &serviceAccountAddress,
			FungibleTokenAddress:  &fungibleTokenAddress,
			FlowTokenAddress:      &flowTokenAddress,
		},
		benchmark.LoadParams{
			NumberOfAccounts: maxInflight,
			LoadType:         benchmark.LoadType(loadType),
			FeedbackEnabled:  feedbackEnabled,
		},
		// We do support only one load type for now.
		benchmark.ConstExecParams{},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to create new cont load generator")
	}

	err = lg.Init()
	if err != nil {
		log.Fatal().Err(err).Msg("unable to init loader")
	}

	// run load
	err = lg.SetTPS(uint(*initialTPSFlag))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to set tps")
	}

	adjuster := NewTPSAdjuster(
		bCtx,
		log,
		lg,
		workerStatsTracker,
		adjustInterval,
		uint(*initialTPSFlag),
		uint(*minTPSFlag),
		uint(*maxTPSFlag),
		uint(maxInflight/2),
	)
	defer adjuster.Stop()

	recorder := NewTPSRecorder(bCtx, workerStatsTracker)
	defer recorder.Stop()

	log.Info().Msg("Waiting for load to finish")
	select {
	case <-time.After(*durationFlag):
	case <-bCtx.Done():
		log.Warn().Err(bCtx.Err()).Msg("loader context canceled")
	}

	log.Info().Msg("Cancelling benchmark context")
	bCancel()
	recorder.Stop()
	adjuster.Stop()

	log.Info().Msg("Stopping load generator")
	lg.Stop()

	log.Info().Msg("Validating data")
	if len(recorder.BenchmarkResults.RawTPS) == 0 {
		recorder.SetStatus(StatusFailure)
	}

	log.Info().Msg("Initializing BigQuery")
	db, err := NewDB(ctx, log, *bigQueryProjectFlag)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create bigquery client")
	}
	defer db.Close()

	err = db.createTable(ctx, *bigQueryDatasetFlag, *bigQueryRawTableFlag, RawRecord{})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create raw TPS table")
	}

	log.Info().Msg("Uploading data to BigQuery")
	err = db.saveRawResults(
		ctx,
		*bigQueryDatasetFlag,
		*bigQueryRawTableFlag,
		recorder.BenchmarkResults,
		*repoInfo,
		BenchmarkInfo{BenchmarkType: loadType},
		defaultEnvironment(),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to send data to bigquery")
	}
}
