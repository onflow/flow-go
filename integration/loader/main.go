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

func main() {
	sleep := flag.Duration("sleep", 0, "duration to sleep before benchmarking starts")
	loadTypeFlag := flag.String("load-type", "token-transfer", "type of loads (\"token-transfer\", \"add-keys\", \"computation-heavy\", \"event-heavy\", \"ledger-heavy\")")
	tpsFlag := flag.String("tps", "1", "transactions per second (TPS) to send, accepts a comma separated list of values if used in conjunction with `tps-durations`")
	tpsDurationsFlag := flag.String("tps-durations", "0", "duration that each load test will run, accepts a comma separted list that will be applied to multiple values of the `tps` flag (defaults to infinite if not provided, meaning only the first tps case will be tested; additional values will be ignored)")
	chainIDStr := flag.String("chain", string(flowsdk.Emulator), "chain ID")
	access := flag.String("access", net.JoinHostPort("127.0.0.1", "3569"), "access node address")
	serviceAccountPrivateKeyHex := flag.String("servPrivHex", unittest.ServiceAccountPrivateKeyHex, "service account private key hex")
	logLvl := flag.String("log-level", "info", "set log level")
	metricport := flag.Uint("metricport", 8080, "port for /metrics endpoint")
	pushgateway := flag.String("pushgateway", "127.0.0.1:9091", "host:port for pushgateway")
	profilerEnabled := flag.Bool("profiler-enabled", false, "whether to enable the auto-profiler")
	_ = flag.Bool("track-txs", false, "deprecated")
	accountMultiplierFlag := flag.Int("account-multiplier", 50, "number of accounts to create per load tps")
	feedbackEnabled := flag.Bool("feedback-enabled", true, "wait for trannsaction execution before submitting new transaction")
	flag.Parse()

	chainID := flowsdk.ChainID([]byte(*chainIDStr))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

			// if the duration is 0, we run this case forever
			if c.duration.Nanoseconds() == 0 {
				return
			}

			preTestTransactionAverage := getPrometheusTotalTransactions()
			lg.StartTime()
			time.Sleep(c.duration)
			lg.StopTime()
			postTestTransactionAverage := getPrometheusTotalTransactions()

			printPrometheusTPS(preTestTransactionAverage, postTestTransactionAverage, c.duration)
			fmt.Println("TPS data finished writing to file.")
			if lg != nil {
				lg.Stop()
			}
		}
	}()

	<-ctx.Done()
}

func getPrometheusTotalTransactions() float64 {
	resp, err := http.Get("http://localhost:9090/api/v1/query?query=execution_runtime_total_executed_transactions")
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

func printPrometheusTPS(preCount float64, postCount float64, duration time.Duration) {
	totalTxs := postCount - preCount
	totalTPS := totalTxs / duration.Seconds()
	f, err := os.OpenFile("avgTps.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		totalTPSstring := fmt.Sprintf(
			"\nOutput TPS:%.4f, Time:%v, Txs:%v",
			totalTPS,
			duration,
			totalTxs)
		f.Write([]byte(totalTPSstring))
		f.Close()
	} else {
		fmt.Println(err)
	}
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
