package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"go.einride.tech/pid"
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
	tps      uint
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
	accountMultiplier           = 50
	feedbackEnabled             = true
	serviceAccountPrivateKeyHex = unittest.ServiceAccountPrivateKeyHex

	//
	// P-controller parameters
	//

	// desiredInflightPerWorker is the target for the controller.
	// Steady state value is ~17. Here we set it higher so that the controller
	// constantly pushes the system slightly out of balance to determine its maximum.
	desiredInflightPerWorker = 30
	// proportionalGain is the proportional gain of the controller.  Lower if controller starts to oscillate.
	proportionalGain = 1.0
	// adjustInterval is the interval between controller updates.  Should be at least 1 RTT of the system, we set it to ~3.
	adjustInterval = 20 * time.Second
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
	initialTPSFlag := flag.Int("initial-tps", 10, "starting transactions per second")
	maxTPSFlag := flag.Int("max-tps", *initialTPSFlag, "maximum transactions per second allowed")
	durationFlag := flag.Duration("duration", 10*time.Minute, "test duration")
	bigQueryProjectFlag := flag.String("bigquery-project", "dapperlabs-data", "project name for the bigquery uploader")
	bigQueryDatasetFlag := flag.String("bigquery-dataset", "dev_src_flow_tps_metrics", "dataset name for the bigquery uploader")
	bigQueryTableFlag := flag.String("bigquery-table", "tpsslices", "table name for the bigquery uploader")
	sliceSize := flag.Duration("slice-size", 2*time.Minute, "the amount of time that each slice covers")
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

	loadCase := LoadCase{tps: uint(*initialTPSFlag), duration: *durationFlag}

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

	// prepare load generator
	log.Info().
		Str("load_type", loadType).
		Uint("tps", loadCase.tps).
		Int("maxTPS", *maxTPSFlag).
		Dur("duration", loadCase.duration).
		Msg("Running load case...")

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
			NumberOfAccounts: *maxTPSFlag * accountMultiplier,
			LoadType:         benchmark.LoadType(loadType),
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
		log.Fatal().Err(err).Msg("unable to create new cont load generator")
	}

	err = lg.Init()
	if err != nil {
		log.Fatal().Err(err).Msg("unable to init loader")
	}

	// run load
	err = lg.SetTPS(loadCase.tps)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to set tps")
	}

	// prepare data slices
	var dataSlices []dataSlice

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		dataSlices = recordTransactionData(
			lg,
			*sliceSize,
			runStartTime,
			gitSha,
			goVersion,
			osVersion)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := adjustTPS(lg, log, adjustInterval, loadCase.tps, uint(*maxTPSFlag))
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Fatal().Err(err).Msgf("unable to adjust tps")
		}
	}()

	select {
	case <-time.After(loadCase.duration):
	case <-ctx.Done():
		// TODO(rbtz): the loader currently doesn't ever cancel the context.
		log.Warn().Err(ctx.Err()).Msg("loader context canceled")
	}

	log.Info().Msg("Stopping load generator")
	lg.Stop()
	log.Info().Msg("Waiting for workers to finish")
	wg.Wait()

	if len(dataSlices) == 0 {
		log.Fatal().Msg("no data slices recorded")
	}

	log.Info().Msg("Uploading data to BigQuery")
	err = sendDataToBigQuery(ctx, *bigQueryProjectFlag, *bigQueryDatasetFlag, *bigQueryTableFlag, dataSlices)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to send data to bigquery")
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
	defer bqClient.Close()

	dataset := bqClient.Dataset(datasetName)
	table := dataset.Table(tableName)

	if err := table.Inserter().Put(ctx, slices); err != nil {
		return fmt.Errorf("failed to insert data: %w", err)
	}
	return nil
}

// adjustTPS tries to find the maximum TPS that the network can handle using a simple PID controller.
func adjustTPS(
	lg *benchmark.ContLoadGenerator,
	log zerolog.Logger,
	interval time.Duration,
	minTPS uint,
	maxTPS uint,
) error {
	lastTs := time.Now()
	lastTxs := uint(lg.GetTxExecuted())

	c := pid.Controller{
		// P controller.
		// We do not use I and D terms becuase they would likely lead to excessive oscillation.
		Config: pid.ControllerConfig{
			ProportionalGain: proportionalGain,
		},
	}

	for {
		select {
		// NOTE: not using a ticker here since adjusting worker count in SetTPS
		// can take a while and lead to uneven feedback intervals.
		case nowTs := <-time.After(interval):
			currentSentTxs := lg.GetTxSent()
			currentTxs := uint(lg.GetTxExecuted())

			timeDiff := nowTs.Sub(lastTs)
			currentTPS := float64(currentTxs-lastTxs) / timeDiff.Seconds()
			desiredTPS := float64(lg.GetTPS())

			inflight := currentSentTxs - int(currentTxs)

			if currentTPS <= math.SmallestNonzeroFloat64 {
				log.Warn().Msg("TPS is <=0, skipping adjustment")
				continue
			}

			currentIPW := float64(inflight) / currentTPS
			c.Update(pid.ControllerInput{
				ReferenceSignal:  desiredInflightPerWorker,
				ActualSignal:     currentIPW,
				SamplingInterval: timeDiff,
			})

			targetIWP := currentIPW + c.State.ControlSignal
			ratio := targetIWP / currentIPW
			// We do not want to adjust TPS below what we already have.
			targetTPS := math.Max(desiredTPS*ratio, currentTPS)
			boundedTPS := boundTPS(targetTPS, minTPS, maxTPS)

			log.Info().
				Float64("currentTPS", currentTPS).
				Float64("unboundedTPS", targetTPS).
				Uint("boundedTPS", boundedTPS).
				Int("inflight", inflight).
				Float64("currentIPW", currentIPW).
				Float64("targetIWP", targetIWP).
				Msg("adjusting TPS")

			err := lg.SetTPS(boundedTPS)
			if err != nil {
				return fmt.Errorf("unable to set tps: %w", err)
			}

			//
			// SetTPS is a blocking call, so we need to re-fetch the TxExecuted and time.
			//
			lastTxs = uint(lg.GetTxExecuted())
			lastTs = time.Now()
		case <-lg.Done():
			return nil
		}
	}
}

// boundTPS bounds the given TPS to the given min and max values.
func boundTPS(tps float64, min uint, max uint) uint {
	switch {
	case tps < float64(min) || math.IsNaN(tps):
		return min
	case tps > float64(max):
		return max
	default:
		return uint(tps)
	}
}
