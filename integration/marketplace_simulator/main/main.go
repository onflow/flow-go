package main

import (
	"encoding/hex"
	"flag"
	"os"
	"strings"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog"

	marketplace "github.com/onflow/flow-go/integration/marketplace_simulator"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func main() {

	sleep := flag.Duration("sleep", 0, "duration to sleep before benchmarking starts")
	logLvl := flag.String("log-level", "info", "set log level")

	// loadTypeFlag := flag.String("load-type", "token-transfer", "type of loads (\"token-transfer\", \"add-keys\", \"computation-heavy\", \"event-heavy\", \"ledger-heavy\")")
	// tpsFlag := flag.String("tps", "1", "transactions per second (TPS) to send, accepts a comma separated list of values if used in conjunction with `tps-durations`")
	// tpsDurationsFlag := flag.String("tps-durations", "0", "duration that each load test will run, accepts a comma separted list that will be applied to multiple values of the `tps` flag (defaults to infinite if not provided, meaning only the first tps case will be tested; additional values will be ignored)")
	chainIDStr := flag.String("chain", string(flowsdk.Testnet), "chain ID")

	access := flag.String("access", "localhost:3569", "access node address")
	serviceAccPrivateKeyHex := flag.String("servPrivHex", unittest.ServiceAccountPrivateKeyHex, "service account private key hex")
	// metricport := flag.Uint("metricport", 8080, "port for /metrics endpoint")
	// profilerEnabled := flag.Bool("profiler-enabled", false, "whether to enable the auto-profiler")

	numberOfAccounts := flag.Int("account-count", 10, "number of accounts")
	// maxTxInFlight := flag.Int("max-tx-in-flight", 8000, "maximum number of transactions in flight (txTracker)")
	// workerDelayAfterEachFetch := flag.Duration("worker-fetch-delay", time.Second, "duration of worker sleep after each tx status fetch (prevents too much requests)")

	flag.Parse()

	// parse log level and apply to logger
	log := zerolog.New(os.Stderr).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
	lvl, err := zerolog.ParseLevel(strings.ToLower(*logLvl))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log = log.Level(lvl)

	chainID := flowsdk.ChainID([]byte(*chainIDStr))

	// TODO metrics
	// server := metrics.NewServer(log, *metricport, *profilerEnabled)
	// <-server.Ready()
	// loaderMetrics := metrics.NewLoaderCollector()

	accessNodeAddrs := strings.Split(*access, ",")

	addressGen := flowsdk.NewAddressGenerator(chainID)
	serviceAccountAddress := addressGen.NextAddress()
	log.Info().Msgf("Service Address: %v", serviceAccountAddress)
	fungibleTokenAddress := addressGen.NextAddress()
	log.Info().Msgf("Fungible Token Address: %v", fungibleTokenAddress)
	flowTokenAddress := addressGen.NextAddress()
	log.Info().Msgf("Flow Token Address: %v", flowTokenAddress)

	serviceAccountPrivateKeyBytes, err := hex.DecodeString(*serviceAccPrivateKeyHex)
	if err != nil {
		log.Fatal().Err(err).Msgf("error while hex decoding hardcoded root key")
	}

	// RLP decode the key
	serviceAccountPrivateKey, err := flow.DecodeAccountPrivateKey(serviceAccountPrivateKeyBytes)
	if err != nil {
		log.Fatal().Err(err).Msgf("error while decoding hardcoded root key bytes")
	}

	// get the private key string
	serviceAccountPrivateKeyHex := hex.EncodeToString(serviceAccountPrivateKey.PrivateKey.Encode())

	// sleep in order to ensure the testnet is up and running
	if *sleep > 0 {
		log.Info().Msgf("Sleeping for %v before starting benchmark", sleep)
		time.Sleep(*sleep)
	}

	networkConfig := &marketplace.NetworkConfig{
		ServiceAccountAddress: &serviceAccountAddress,
		FlowTokenAddress:      &flowTokenAddress,
		FungibleTokenAddress:  &fungibleTokenAddress,
		// NonFungibleTokenAddress     ???
		ServiceAccountPrivateKeyHex: serviceAccountPrivateKeyHex,
		AccessNodeAddresses:         accessNodeAddrs,
	}

	simulatorConfig := &marketplace.SimulatorConfig{
		NumberOfAccounts: *numberOfAccounts,
		NumberOfMoments:  1000,
		Delay:            time.Second,
	}

	// accessNodeAddrs
	go func() {
		var m *marketplace.MarketPlaceSimulator
		m = marketplace.NewMarketPlaceSimulator(
			log,
			networkConfig,
			simulatorConfig)
		m.Run()
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}
