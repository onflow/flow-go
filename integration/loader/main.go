package main

import (
	"encoding/hex"
	"flag"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

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
	access := flag.String("access", "localhost:3569", "access node address")
	serviceAccountPrivateKeyHex := flag.String("servPrivHex", unittest.ServiceAccountPrivateKeyHex, "service account private key hex")
	logLvl := flag.String("log-level", "info", "set log level")
	metricport := flag.Uint("metricport", 8080, "port for /metrics endpoint")
	profilerEnabled := flag.Bool("profiler-enabled", false, "whether to enable the auto-profiler")
	flag.Parse()

	chainID := flowsdk.ChainID([]byte(*chainIDStr))

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
	flowClient, err := client.New(loadedAccessAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to initialize Flow client")
	}

	supervisorAccessAddr := accessNodeAddrs[0]
	if len(accessNodeAddrs) > 1 {
		supervisorAccessAddr = accessNodeAddrs[1]
	}
	supervisorClient, err := client.New(supervisorAccessAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to initialize Flow supervisor client")
	}

	go func() {
		// run load cases
		for i, c := range cases {
			log.Info().Int("number", i).Int("tps", c.tps).Dur("duration", c.duration).Msgf("Running load case...")

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
					utils.LoadType(*loadTypeFlag),
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

			time.Sleep(c.duration)

			if lg != nil {
				lg.Stop()
			}
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
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
