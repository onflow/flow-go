package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowsdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/integration/utils"
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
	GitSha              string
	StartTime           time.Time
	EndTime             time.Time
	InputTps            float64
	OutputTps           float64
	ProStartTransaction float64
	ProEndTransaction   float64
}

func main() {
	// holdover flags from loader/main.go
	sleep := flag.Duration("sleep", 0, "duration to sleep before benchmarking starts")
	loadTypeFlag := flag.String("load-type", "token-transfer", "type of loads (\"token-transfer\", \"add-keys\", \"computation-heavy\", \"event-heavy\", \"ledger-heavy\", \"const-exec\")")
	chainIDStr := flag.String("chain", string(flowsdk.Emulator), "chain ID")
	access := flag.String("access", net.JoinHostPort("127.0.0.1", "3569"), "access node address")
	serviceAccountPrivateKeyHex := flag.String("servPrivHex", unittest.ServiceAccountPrivateKeyHex, "service account private key hex")
	logLvl := flag.String("log-level", "info", "set log level")
	metricport := flag.Uint("metricport", 8080, "port for /metrics endpoint")
	pushgateway := flag.String("pushgateway", "127.0.0.1:9091", "host:port for pushgateway")
	profilerEnabled := flag.Bool("profiler-enabled", false, "whether to enable the auto-profiler")
	accountMultiplierFlag := flag.Int("account-multiplier", 50, "number of accounts to create per load tps")
	feedbackEnabled := flag.Bool("feedback-enabled", true, "wait for trannsaction execution before submitting new transaction")
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
	gitSha := flag.String("gitSha", "", "the git hash of the run, used for BigQuery result tracking")
	flag.Parse()

	chainID := flowsdk.ChainID([]byte(*chainIDStr))

	ctx, cancel := context.WithCancel(context.Background())

	// parse log level and apply to logger
	log := zerolog.New(os.Stderr).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
	lvl, err := zerolog.ParseLevel(strings.ToLower(*logLvl))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log = log.Level(lvl)

	server := metrics.NewServer(log, *metricport, *profilerEnabled)
	<-server.Ready()
	loaderMetrics := metrics.NewLoaderCollector()

	if *pushgateway != "" {
		pusher := push.New(*pushgateway, "loader").Gatherer(prometheus.DefaultGatherer)
		go func() {
			t := time.NewTicker(10 * time.Second)
			defer t.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					err := pusher.Push()
					if err != nil {
						log.Warn().Err(err).Msg("failed to push metrics to pushgateway")
					}
				}
			}
		}()
	}

	accessNodeAddrs := strings.Split(*access, ",")

	cases := parseLoadCases(log, tpsFlag, tpsDurationsFlag)

	addressGen := flowsdk.NewAddressGenerator(chainID)
	serviceAccountAddress := addressGen.NextAddress()
	log.Info().Msgf("Service Address: %v", serviceAccountAddress)
	fungibleTokenAddress := addressGen.NextAddress()
	log.Info().Msgf("Fungible Token Address: %v", fungibleTokenAddress)
	flowTokenAddress := addressGen.NextAddress()
	log.Info().Msgf("Flow Token Address: %v", flowTokenAddress)

	serviceAccountPrivateKeyBytes, err := hex.DecodeString(*serviceAccountPrivateKeyHex)
	if err != nil {
		log.Fatal().Err(err).Msgf("error while hex decoding hardcoded root key")
	}

	ServiceAccountPrivateKey := flow.AccountPrivateKey{
		SignAlgo: unittest.ServiceAccountPrivateKeySignAlgo,
		HashAlgo: unittest.ServiceAccountPrivateKeyHashAlgo,
	}
	ServiceAccountPrivateKey.PrivateKey, err = crypto.DecodePrivateKey(
		ServiceAccountPrivateKey.SignAlgo, serviceAccountPrivateKeyBytes)
	if err != nil {
		log.Fatal().Err(err).Msgf("error while decoding hardcoded root key bytes")
	}

	// get the private key string
	priv := hex.EncodeToString(ServiceAccountPrivateKey.PrivateKey.Encode())

	// sleep in order to ensure the testnet is up and running
	if *sleep > 0 {
		log.Info().Msgf("Sleeping for %v before starting benchmark", sleep)
		time.Sleep(*sleep)
	}

	loadedAccessAddr := accessNodeAddrs[0]
	flowClient, err := client.NewClient(loadedAccessAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to initialize Flow client")
	}

	supervisorAccessAddr := accessNodeAddrs[0]
	if len(accessNodeAddrs) > 1 {
		supervisorAccessAddr = accessNodeAddrs[1]
	}
	supervisorClient, err := client.NewClient(supervisorAccessAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to initialize Flow supervisor client")
	}

	go func() {
		// run load cases
		for i, c := range cases {
			log.Info().Str("load_type", *loadTypeFlag).Int("number", i).Int("tps", c.tps).Dur("duration", c.duration).Msgf("Running load case...")

			loaderMetrics.SetTPSConfigured(c.tps)

			var lg *utils.ContLoadGenerator
			if c.tps > 0 {
				var err error
				lg, err = utils.NewContLoadGenerator(
					log,
					loaderMetrics,
					flowClient,
					supervisorClient,
					loadedAccessAddr,
					priv,
					&serviceAccountAddress,
					&fungibleTokenAddress,
					&flowTokenAddress,
					c.tps,
					*accountMultiplierFlag,
					utils.LoadType(*loadTypeFlag),
					*feedbackEnabled,
					utils.ConstExecParam{
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
			}

			testStartTime := time.Now()
			time.Sleep(c.duration)
			testEndTime := time.Now()

			if lg != nil {
				lg.Stop()
			}

			dataSlices := calculateTpsSlices(testStartTime, testEndTime, *leadTime, *sliceSize, *gitSha, lg, log)
			prepareDataForBigQuery(dataSlices, *ciFlag)
		}

		cancel()
	}()

	<-ctx.Done()
}

func calculateTpsSlices(start, end time.Time, leadTime, sliceTime string, commit string, lg *utils.ContLoadGenerator, log zerolog.Logger) []dataSlice {
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
		f, err := os.Create("tps-bq-results.json")
		if err == nil {
			f.Write([]byte(jsonString))
			f.Close()
		} else {
			fmt.Println(err)
		}
	}

	// output human-readable json blob
	timestamp := time.Now()
	fileName := fmt.Sprintf("tps-results-%v.json", timestamp)
	f, err := os.Create(fileName)
	if err == nil {
		f.Write(jsonText)
		f.Close()
	} else {
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

func parseLoadCases(log zerolog.Logger, tpsFlag, tpsDurationsFlag *string) []LoadCase {
	tpsStrings := strings.Split(*tpsFlag, ",")
	var cases []LoadCase
	for _, s := range tpsStrings {
		t, err := strconv.ParseInt(s, 0, 32)
		if err != nil {
			log.Fatal().Err(err).Str("value", s).
				Msg("could not parse tps flag, expected comma separated list of integers")
		}
		cases = append(cases, LoadCase{tps: int(t)})
	}

	tpsDurationsStrings := strings.Split(*tpsDurationsFlag, ",")
	for i := range cases {
		if i >= len(tpsDurationsStrings) {
			break
		}

		// ignore empty entries (implying that case will run indefinitely)
		if tpsDurationsStrings[i] == "" {
			continue
		}

		d, err := time.ParseDuration(tpsDurationsStrings[i])
		if err != nil {
			log.Fatal().Err(err).Str("value", tpsDurationsStrings[i]).
				Msg("could not parse tps-durations flag, expected comma separated list of durations")
		}
		cases[i].duration = d
	}

	return cases
}
