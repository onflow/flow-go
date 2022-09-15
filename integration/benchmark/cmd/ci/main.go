package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
	client "github.com/onflow/flow-go-sdk/access/grpc"

	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/integration/benchmark"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

type LoadCase struct {
	tps      int
	duration time.Duration
}

// This struct is used for uploading data to BigQuery.
type dataSlice struct {
	GoVersion           string    `bigquery:"goVersion"`
	OsVersion           string    `bigquery:"osVersion"`
	GitSha              string    `bigquery:"gitSha"`
	StartTime           time.Time `bigquery:"startTime"`
	EndTime             time.Time `bigquery:"endTime"`
	InputTps            float64   `bigquery:"inputTps"`
	OutputTps           float64   `bigquery:"outputTps"`
	StartExecutionCount int       `bigquery:"startExecutionCount"`
	EndExecutionCount   int       `bigquery:"endExecutionCount"`
	RunStartTime        time.Time `bigquery:"runStartTime"`
}

// Hardcoded CI values
const (
	loadType                    = "token-transfer"
	metricport                  = uint(8080)
	accessNodeAddress           = "127.0.0.1:3569"
	pushgateway                 = "127.0.0.1:9091"
	accountMultiplier           = 100
	feedbackEnabled             = true
	serviceAccountPrivateKeyHex = unittest.ServiceAccountPrivateKeyHex
)

func main() {
	// holdover flags from loader/main.go
	logLvl := flag.String("log-level", "info", "set log level")
	profilerEnabled := flag.Bool("profiler-enabled", false, "whether to enable the auto-profiler")
	maxConstExecTxSizeInBytes := flag.Uint("const-exec-max-tx-size", flow.DefaultMaxTransactionByteSize/10, "max byte size of constant exec transaction size to generate")
	authAccNumInConstExecTx := flag.Uint("const-exec-num-authorizer", 1, "num of authorizer for each constant exec transaction to generate")
	argSizeInByteInConstExecTx := flag.Uint("const-exec-arg-size", 100, "byte size of tx argument for each constant exec transaction to generate")
	payerKeyCountInConstExecTx := flag.Uint("const-exec-payer-key-count", 2, "num of payer keys for each constant exec transaction to generate")

	// CI relevant flags
	tpsFlag := flag.String("tps", "300", "transactions per second (TPS) to send, accepts a comma separated list of values if used in conjunction with `tps-durations`")
	tpsDurationsFlag := flag.String("tps-durations", "10m", "duration that each load test will run, accepts a comma separted list that will be applied to multiple values of the `tps` flag (defaults to infinite if not provided, meaning only the first tps case will be tested; additional values will be ignored)")
	_ = flag.Bool("ci-run", false, "(deprecated)")
	bigQueryProjectFlag := flag.String("bigquery_project", "dapperlabs-data", "project name for the bigquery uploader")
	bigQueryDatasetFlag := flag.String("bigquery_dataset", "dev_src_flow_tps_metrics", "dataset name for the bigquery uploader")
	bigQueryTableFlag := flag.String("bigquery_table", "tpsslices", "table name for the bigquery uploader")
	sliceSize := flag.String("slice-size", "2m", "the amount of time that each slice covers")
	flag.Parse()

	// Version and Commit Info
	gitSha := build.Commit()
	goVersion := runtime.Version()
	osVersion := runtime.GOOS + runtime.GOARCH

	runStartTime := time.Now()
	if gitSha == "undefined" {
		gitSha = runStartTime.String()
	}

	chainID := flowsdk.Emulator

	// parse log level and apply to logger
	log := zerolog.New(os.Stderr).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
	lvl, err := zerolog.ParseLevel(strings.ToLower(*logLvl))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log = log.Level(lvl)

	server := metrics.NewServer(log, metricport, *profilerEnabled)
	<-server.Ready()
	loaderMetrics := metrics.NewLoaderCollector()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sp := benchmark.NewStatsPusher(ctx, log, pushgateway, "loader", prometheus.DefaultGatherer)
	defer sp.Stop()

	tps, err := strconv.ParseInt(*tpsFlag, 0, 32)
	if err != nil {
		log.Fatal().Err(err).Str("value", *tpsFlag).
			Msg("could not parse tps flag")
	}
	tpsDuration, err := time.ParseDuration(*tpsDurationsFlag)
	if err != nil {
		log.Fatal().Err(err).Str("value", *tpsDurationsFlag).
			Msg("could not parse tps-durations flag")
	}
	loadCase := LoadCase{tps: int(tps), duration: tpsDuration}

	addressGen := flowsdk.NewAddressGenerator(chainID)
	serviceAccountAddress := addressGen.NextAddress()
	log.Info().Msgf("Service Address: %v", serviceAccountAddress)
	fungibleTokenAddress := addressGen.NextAddress()
	log.Info().Msgf("Fungible Token Address: %v", fungibleTokenAddress)
	flowTokenAddress := addressGen.NextAddress()
	log.Info().Msgf("Flow Token Address: %v", flowTokenAddress)

	flowClient, err := client.NewClient(accessNodeAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to initialize Flow client")
	}

	// prepare load generator
	log.Info().
		Str("load_type", loadType).
		Int("tps", loadCase.tps).
		Dur("duration", loadCase.duration).
		Msgf("Running load case...")

	loaderMetrics.SetTPSConfigured(loadCase.tps)

	lg, err := benchmark.New(
		ctx,
		log,
		loaderMetrics,
		[]access.Client{flowClient},
		benchmark.NetworkParams{
			ServAccPrivKeyHex:     serviceAccountPrivateKeyHex,
			ServiceAccountAddress: &serviceAccountAddress,
			FungibleTokenAddress:  &fungibleTokenAddress,
			FlowTokenAddress:      &flowTokenAddress,
		},
		benchmark.LoadParams{
			TPS:              loadCase.tps,
			NumberOfAccounts: loadCase.tps * accountMultiplier,
			LoadType:         loadType,
			FeedbackEnabled:  feedbackEnabled,
		},
		benchmark.ConstExecParams{
			MaxTxSizeInByte: *maxConstExecTxSizeInBytes,
			AuthAccountNum:  *authAccNumInConstExecTx,
			ArgSizeInByte:   *argSizeInByteInConstExecTx,
			PayerKeyCount:   *payerKeyCountInConstExecTx,
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to create new cont load generator")
	}

	err = lg.Init()
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to init loader")
	}

	// run load
	lg.Start()

	// prepare data slices
	sliceDuration, _ := time.ParseDuration(*sliceSize)
	var dataSlices []dataSlice

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		dataSlices = recordTransactionData(
			lg,
			sliceDuration,
			runStartTime,
			gitSha,
			goVersion,
			osVersion)
	}()

	select {
	case <-time.After(loadCase.duration):
	case <-ctx.Done(): //TODO: the loader currently doesn't ever cancel the context
		// add logging here to express the *why* of lg choose to cancel the context.
		// when the loader cancels its own context it may also call Stop() on itself
		// this may become redundant.
	}
	lg.Stop()
	wg.Wait()

	err = sendDataToBigQuery(ctx, *bigQueryProjectFlag, *bigQueryDatasetFlag, *bigQueryTableFlag, dataSlices)
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to send data to bigquery")
	}
}

func recordTransactionData(
	lg *benchmark.ContLoadGenerator,
	sliceDuration time.Duration,
	runStartTime time.Time,
	gitSha, goVersion, osVersion string,
) []dataSlice {
	var dataSlices []dataSlice

	// get initial values for first slice
	startTime := time.Now()
	startExecutedTransactions := lg.GetTxExecuted()

	t := time.NewTicker(sliceDuration)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			endTime := time.Now()
			endExecutedTransaction := lg.GetTxExecuted()

			// calculate this slice
			inputTps := lg.AvgTpsBetween(startTime, endTime)
			outputTps := float64(endExecutedTransaction-startExecutedTransactions) / sliceDuration.Seconds()
			dataSlices = append(dataSlices,
				dataSlice{
					GitSha:              gitSha,
					GoVersion:           goVersion,
					OsVersion:           osVersion,
					StartTime:           startTime,
					EndTime:             endTime,
					InputTps:            inputTps,
					OutputTps:           outputTps,
					StartExecutionCount: startExecutedTransactions,
					EndExecutionCount:   endExecutedTransaction,
					RunStartTime:        runStartTime,
				})

			// set start values for next slice
			startExecutedTransactions = endExecutedTransaction
			startTime = endTime
		case <-lg.Done():
			return dataSlices
		}
	}
}

func sendDataToBigQuery(
	ctx context.Context,
	projectName, datasetName, tableName string,
	slices []dataSlice,
) error {
	bqClient, err := bigquery.NewClient(ctx, projectName)
	if err != nil {
		return fmt.Errorf("unable to create bigquery client: %w", err)
	}

	dataset := bqClient.Dataset(datasetName)
	table := dataset.Table(tableName)

	if err := table.Inserter().Put(ctx, slices); err != nil {
		return fmt.Errorf("failed to insert data: %w", err)
	}
	return nil
}
