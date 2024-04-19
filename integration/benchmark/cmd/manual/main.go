package main

import (
	"context"
	"flag"
	"net"
	"os"
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

	"github.com/onflow/flow-go/integration/benchmark"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

type LoadCase struct {
	tps      uint
	duration time.Duration
}

const (
	defaultMaxMsgSize = 1024 * 1024 * 16 // 16 MB
)

func main() {
	sleep := flag.Duration("sleep", 0, "duration to sleep before benchmarking starts")
	loadTypeFlag := flag.String("load-type", "token-transfer", "type of loads (\"token-transfer\", \"add-keys\", \"computation-heavy\", \"event-heavy\", \"ledger-heavy\", \"const-exec\", \"exec-data-heavy\")")
	tpsFlag := flag.String("tps", "1", "transactions per second (TPS) to send, accepts a comma separated list of values if used in conjunction with `tps-durations`")
	tpsDurationsFlag := flag.String("tps-durations", "0", "duration that each load test will run, accepts a comma separted list that will be applied to multiple values of the `tps` flag (defaults to infinite if not provided, meaning only the first tps case will be tested; additional values will be ignored)")
	chainIDStr := flag.String("chain", string(flowsdk.Emulator), "chain ID")
	accessNodes := flag.String("access", net.JoinHostPort("127.0.0.1", "4001"), "access node address")
	serviceAccountPrivateKeyHex := flag.String("servPrivHex", unittest.ServiceAccountPrivateKeyHex, "service account private key hex")
	logLvl := flag.String("log-level", "info", "set log level")
	metricport := flag.Uint("metricport", 8080, "port for /metrics endpoint")
	pushgateway := flag.String("pushgateway", "127.0.0.1:9091", "host:port for pushgateway")
	_ = flag.Bool("track-txs", false, "deprecated")
	accountMultiplierFlag := flag.Int("account-multiplier", 100, "number of accounts to create per load tps")
	feedbackEnabled := flag.Bool("feedback-enabled", true, "wait for trannsaction execution before submitting new transaction")
	flag.Parse()

	chainID := flowsdk.ChainID(*chainIDStr)

	// parse log level and apply to logger
	log := zerolog.New(os.Stderr).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
	lvl, err := zerolog.ParseLevel(strings.ToLower(*logLvl))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log = log.Level(lvl)

	server := metrics.NewServer(log, *metricport)
	<-server.Ready()
	loaderMetrics := metrics.NewLoaderCollector()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *pushgateway != "disabled" {
		sp := benchmark.NewStatsPusher(ctx, log, *pushgateway, "loader", prometheus.DefaultGatherer)
		defer sp.Stop()
	}

	addressGen := flowsdk.NewAddressGenerator(chainID)
	serviceAccountAddress := addressGen.NextAddress()
	log.Info().Msgf("Service Address: %v", serviceAccountAddress)
	fungibleTokenAddress := addressGen.NextAddress()
	log.Info().Msgf("Fungible Token Address: %v", fungibleTokenAddress)
	flowTokenAddress := addressGen.NextAddress()
	log.Info().Msgf("Flow Token Address: %v", flowTokenAddress)

	// sleep in order to ensure the testnet is up and running
	if *sleep > 0 {
		log.Info().Msgf("Sleeping for %v before starting benchmark", sleep)
		time.Sleep(*sleep)
	}

	accessNodeAddrs := strings.Split(*accessNodes, ",")
	clients := make([]access.Client, 0, len(accessNodeAddrs))
	for _, addr := range accessNodeAddrs {
		client, err := client.NewClient(
			addr,
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(defaultMaxMsgSize),
				grpc.MaxCallSendMsgSize(defaultMaxMsgSize),
			),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatal().Str("addr", addr).Err(err).Msgf("unable to initialize flow client")
		}
		clients = append(clients, client)
	}

	// run load cases and compute max tps
	var maxTPS uint
	loadCases := parseLoadCases(log, tpsFlag, tpsDurationsFlag)
	for _, c := range loadCases {
		if c.tps > maxTPS {
			maxTPS = c.tps
		}
	}

	workerStatsTracker := benchmark.NewWorkerStatsTracker(ctx)
	defer workerStatsTracker.Stop()

	statsLogger := benchmark.NewPeriodicStatsLogger(context.TODO(), workerStatsTracker, log)
	statsLogger.Start()
	defer statsLogger.Stop()

	lg, err := benchmark.New(
		ctx,
		log,
		workerStatsTracker,
		loaderMetrics,
		clients,
		benchmark.NetworkParams{
			ServAccPrivKeyHex: *serviceAccountPrivateKeyHex,
			ChainId:           flow.ChainID(chainID),
		},
		benchmark.LoadParams{
			NumberOfAccounts: int(maxTPS) * *accountMultiplierFlag,
			LoadConfig: benchmark.LoadConfig{
				LoadName:   *loadTypeFlag,
				LoadType:   *loadTypeFlag,
				TpsMax:     int(maxTPS),
				TpsMin:     int(maxTPS),
				TPSInitial: int(maxTPS),
			},
			FeedbackEnabled: *feedbackEnabled,
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to create new cont load generator")
	}
	defer lg.Stop()

	for i, c := range loadCases {
		log.Info().
			Str("load_type", *loadTypeFlag).
			Int("number", i).
			Uint("tps", c.tps).
			Dur("duration", c.duration).
			Msg("running load case")

		err = lg.SetTPS(c.tps)
		if err != nil {
			log.Fatal().Err(err).Msg("unable to set tps")
		}

		// if the duration is 0, we run this case forever
		waitC := make(<-chan time.Time)
		if c.duration != 0 {
			waitC = time.After(c.duration)
		}

		select {
		case <-ctx.Done():
			log.Info().Err(ctx.Err()).Msg("context cancelled")
			return
		case <-waitC:
			log.Info().Msg("finished load case")
		}
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
		cases = append(cases, LoadCase{tps: uint(t)})
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
