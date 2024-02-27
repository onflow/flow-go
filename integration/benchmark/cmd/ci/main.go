package main

import (
	"context"
	"flag"
	"net"
	"os"
	"strings"
	"time"

	"github.com/onflow/flow-go/integration/benchmark/load"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
	client "github.com/onflow/flow-go-sdk/access/grpc"

	"github.com/onflow/flow-go/integration/benchmark"
	pb "github.com/onflow/flow-go/integration/benchmark/proto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

type BenchmarkInfo struct {
	BenchmarkType string
}

// Hardcoded CI values
const (
	defaultLoadType             = "token-transfer"
	metricport                  = uint(8080)
	accessNodeAddress           = "127.0.0.1:4001"
	pushgateway                 = "127.0.0.1:9091"
	accountMultiplier           = 50
	feedbackEnabled             = true
	serviceAccountPrivateKeyHex = unittest.ServiceAccountPrivateKeyHex

	defaultAdjustInterval           = 10 * time.Second
	defaultMetricCollectionInterval = 20 * time.Second

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
	adjustIntervalFlag := flag.Duration("tps-adjust-interval", defaultAdjustInterval, "interval for adjusting TPS")
	adjustDelayFlag := flag.Duration("tps-adjust-delay", 120*time.Second, "delay before adjusting TPS")
	statIntervalFlag := flag.Duration("stat-interval", defaultMetricCollectionInterval, "")
	durationFlag := flag.Duration("duration", 10*time.Minute, "test duration")
	gitRepoPathFlag := flag.String("git-repo-path", "../..", "git repo path of the filesystem")
	gitRepoURLFlag := flag.String("git-repo-url", "https://github.com/onflow/flow-go.git", "git repo URL")
	bigQueryUpload := flag.Bool("bigquery-upload", true, "whether to upload results to BigQuery (true / false)")
	bigQueryProjectFlag := flag.String("bigquery-project", "dapperlabs-data", "project name for the bigquery uploader")
	bigQueryDatasetFlag := flag.String("bigquery-dataset", "dev_src_flow_tps_metrics", "dataset name for the bigquery uploader")
	bigQueryRawTableFlag := flag.String("bigquery-raw-table", "rawResults", "table name for the bigquery raw results")
	loadTypeFlag := flag.String("load-type", defaultLoadType, "load type (token-transfer / const-exec / evm)")
	flag.Parse()

	loadType := *loadTypeFlag

	log := setupLogger(logLvl)

	if *gitRepoPathFlag == "" {
		flag.PrintDefaults()
		log.Fatal().Msg("git repo path is required")
	}

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

	addressGen := flowsdk.NewAddressGenerator(flowsdk.Emulator)
	serviceAccountAddress := addressGen.NextAddress()
	fungibleTokenAddress := addressGen.NextAddress()
	flowTokenAddress := addressGen.NextAddress()
	log.Info().
		Stringer("serviceAccountAddress", serviceAccountAddress).
		Stringer("fungibleTokenAddress", fungibleTokenAddress).
		Stringer("flowTokenAddress", flowTokenAddress).
		Msg("addresses")

	flowClient, err := client.NewClient(
		accessNodeAddress,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(defaultMaxMsgSize),
			grpc.MaxCallSendMsgSize(defaultMaxMsgSize),
		),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
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

	statsLogger := benchmark.NewPeriodicStatsLogger(ctx, workerStatsTracker, log)
	statsLogger.Start()
	defer statsLogger.Stop()

	lg, err := benchmark.New(
		bCtx,
		log,
		workerStatsTracker,
		loaderMetrics,
		[]access.Client{flowClient},
		benchmark.NetworkParams{
			ServAccPrivKeyHex: serviceAccountPrivateKeyHex,
			ChainId:           flow.Emulator,
		},
		benchmark.LoadParams{
			NumberOfAccounts: maxInflight,
			LoadType:         load.LoadType(loadType),
			FeedbackEnabled:  feedbackEnabled,
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to create new cont load generator")
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

		AdjusterParams{
			Delay:       *adjustDelayFlag,
			Interval:    *adjustIntervalFlag,
			InitialTPS:  uint(*initialTPSFlag),
			MinTPS:      uint(*minTPSFlag),
			MaxTPS:      uint(*maxTPSFlag),
			MaxInflight: uint(maxInflight / 2),
		},
	)
	defer adjuster.Stop()

	recorder := NewTPSRecorder(bCtx, workerStatsTracker, *statIntervalFlag)
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

	mustValidateData(log, recorder)

	// only upload valid data
	if *bigQueryUpload {
		repoInfo := MustGetRepoInfo(log, *gitRepoURLFlag, *gitRepoPathFlag)
		mustUploadData(ctx, log, recorder, repoInfo, *bigQueryProjectFlag, *bigQueryDatasetFlag, *bigQueryRawTableFlag, loadType)
	} else {
		log.Info().Int("raw_tps_size", len(recorder.BenchmarkResults.RawTPS)).Msg("logging tps results locally")
		// log results locally when not uploading to BigQuery
		for i, tpsRecord := range recorder.BenchmarkResults.RawTPS {
			log.Info().Int("tps_record_index", i).Interface("tpsRecord", tpsRecord).Msg("tps_record")
		}
	}
}

// setupLogger parses log level and apply to logger
func setupLogger(logLvl *string) zerolog.Logger {
	log := zerolog.New(os.Stderr).
		With().
		Timestamp().
		Logger().
		Output(zerolog.ConsoleWriter{Out: os.Stderr})

	lvl, err := zerolog.ParseLevel(strings.ToLower(*logLvl))
	if err != nil {
		log.Fatal().Err(err).Str("strLevel", *logLvl).Msg("invalid log level")
	}
	log = log.Level(lvl)
	return log
}

func mustUploadData(
	ctx context.Context,
	log zerolog.Logger,
	recorder *TPSRecorder,
	repoInfo *RepoInfo,
	bigQueryProject string,
	bigQueryDataset string,
	bigQueryRawTable string,
	loadType string,
) {
	log.Info().Msg("Initializing BigQuery")
	db, err := NewDB(ctx, log, bigQueryProject)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create bigquery client")
	}
	defer func(db *DB) {
		err := db.Close()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to close bigquery client")
		}
	}(db)

	err = db.createTable(ctx, bigQueryDataset, bigQueryRawTable, RawRecord{})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create raw TPS table")
	}

	log.Info().Msg("Uploading data to BigQuery")
	err = db.saveRawResults(
		ctx,
		bigQueryDataset,
		bigQueryRawTable,
		recorder.BenchmarkResults,
		*repoInfo,
		BenchmarkInfo{BenchmarkType: loadType},
		MustGetDefaultEnvironment(),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to send data to bigquery")
	}
}

func mustValidateData(log zerolog.Logger, recorder *TPSRecorder) {
	log.Info().Msg("Validating data")
	var totalTPS float64
	for _, record := range recorder.BenchmarkResults.RawTPS {
		totalTPS += record.OutputTPS
	}
	resultLen := len(recorder.BenchmarkResults.RawTPS)
	if resultLen == 0 || totalTPS == 0 {
		recorder.SetStatus(StatusFailure)
		log.Fatal().
			Int("resultsLen", resultLen).
			Float64("totalTPS", totalTPS).
			Msg("no TPS data generated")
	}
}
