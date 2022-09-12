package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

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

// This struct is used for uploading data to BigQuery, changes here should
// remain in sync with tps_results_schema.json
type dataSlice struct {
	GoVersion           string
	OsVersion           string
	GitSha              string
	StartTime           time.Time
	EndTime             time.Time
	InputTps            float64
	OutputTps           float64
	ProStartTransaction float64
	ProEndTransaction   float64
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
	ciFlag := flag.Bool("ci-run", false, "whether or not the run is part of CI")
	leadTime := flag.String("leadTime", "30s", "the amount of time before data slices are started")
	sliceSize := flag.String("sliceSize", "2m", "the amount of time that each slice covers")
	flag.Parse()

	// Version and Commit Info
	gitSha := build.Commit()
	goVersion := runtime.Version()
	osVersion := runtime.GOOS + runtime.GOARCH

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

	// run load
	log.Info().Str("load_type", loadType).Int("tps", loadCase.tps).Dur("duration", loadCase.duration).Msgf("Running load case...")

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
		log.Fatal().Err(err).Msgf("unable to create new cont load generator")
	}

	err = lg.Init()
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to init loader")
	}
	lg.Start()

	testStartTime := time.Now()
	time.Sleep(loadCase.duration)
	lg.Stop()
	testEndTime := time.Now()

	dataSlices := calculateTpsSlices(
		testStartTime,
		testEndTime,
		*leadTime,
		*sliceSize,
		gitSha,
		goVersion,
		osVersion,
		lg)
	prepareDataForBigQuery(dataSlices, *ciFlag)
}

func calculateTpsSlices(start, end time.Time, leadTime, sliceTime, commit, goVersion, osVersion string, lg *benchmark.ContLoadGenerator) []dataSlice {
	//remove the lead time on both start and end, this should remove spin-up and spin-down times
	leadDuration, _ := time.ParseDuration(leadTime)
	endTime := end.Add(-1 * leadDuration)
	sliceDuration, _ := time.ParseDuration(sliceTime)

	slices := make([]dataSlice, 0)

	for currentTime := start.Add(leadDuration); currentTime.Add(sliceDuration).Before(endTime); currentTime = currentTime.Add(sliceDuration) {
		sliceEndTime := currentTime.Add(sliceDuration)

		inputTps := lg.AvgTpsBetween(currentTime, sliceEndTime)
		proStart := getPrometheusTransactionAtTime(currentTime)
		proEnd := getPrometheusTransactionAtTime(sliceEndTime)

		outputTps := (proEnd - proStart) / (sliceDuration).Seconds()

		slice := dataSlice{
			GitSha:              commit,
			GoVersion:           goVersion,
			OsVersion:           osVersion,
			StartTime:           currentTime,
			EndTime:             currentTime.Add(sliceDuration),
			InputTps:            inputTps,
			OutputTps:           outputTps,
			ProStartTransaction: proStart,
			ProEndTransaction:   proEnd}

		slices = append(slices, slice)
	}

	return slices
}

func prepareDataForBigQuery(slices []dataSlice, ci bool) {
	jsonText, err := json.Marshal(slices)
	if err != nil {
		println("Error converting slice data to json")
	}

	// if we are uploading to BigQuery then we need a copy of the json file
	// that is newline delimited json
	if ci {
		jsonString := string(jsonText)
		// remove the surrounding square brackets
		jsonString = jsonString[1 : len(jsonString)-1]
		// end of object commas will be replaced with newlines
		jsonString = strings.ReplaceAll(jsonString, "},", "}\n")

		// output tps-bq-results
		err = os.WriteFile("tps-bq-results.json", []byte(jsonString), 0666)
		if err != nil {
			fmt.Println(err)
		}
	}

	// output human-readable json blob
	timestamp := time.Now()
	fileName := fmt.Sprintf("tps-results-%v.json", timestamp)
	err = os.WriteFile(fileName, jsonText, 0666)
	if err != nil {
		fmt.Println(err)
	}
}

func getPrometheusTransactionAtTime(time time.Time) float64 {
	url := fmt.Sprintf("http://localhost:9090/api/v1/query?query=execution_runtime_total_executed_transactions&time=%v", time.Unix())
	resp, err := http.Get(url)
	if err != nil {
		// error handling
		println("Error getting prometheus data")
		return -1
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body, %v", err)
		return -1
	}

	var result map[string]interface{}

	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		fmt.Printf("Error unmarshaling json, %v", err)
		return -1
	}

	totalTxs := 0
	executionNodeCount := 0

	resultMap := result["data"].(map[string]interface{})["result"].([]interface{})

	for i, executionNodeMap := range resultMap {
		executionNodeCount = i
		nodeMap, _ := executionNodeMap.(map[string]interface{})
		values := nodeMap["value"].([]interface{})
		nodeTxsStr := values[1].(string)
		nodeTxsInt, _ := strconv.Atoi(nodeTxsStr)
		totalTxs = totalTxs + nodeTxsInt
	}

	if executionNodeCount == 0 {
		println("No execution nodes found. No transactions.")
		return 0
	}

	avgTps := float64(totalTxs) / float64(executionNodeCount)
	return avgTps
}
