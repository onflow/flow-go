package main

import (
	"encoding/hex"
	"flag"
	"os"
	"strings"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/integration/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func main() {

	sleep := flag.Duration("sleep", 0, "duration to sleep before benchmarking starts")
	tps := flag.Int("tps", 100, "transactions to send per second")
	chainIDStr := flag.String("chain", string(flowsdk.Testnet), "chain ID")
	chainID := flowsdk.ChainID([]byte(*chainIDStr))
	access := flag.String("access", "localhost:3569", "access node address")
	serviceAccountPrivateKeyHex := flag.String("servPrivHex", unittest.ServiceAccountPrivateKeyHex, "service account private key hex")
	logLvl := flag.String("log-level", "info", "set log level")
	flag.Parse()

	// parse log level and apply to logger
	log := zerolog.New(os.Stderr).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
	lvl, err := zerolog.ParseLevel(strings.ToLower(*logLvl))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log = log.Level(lvl)

	accessNodeAddrs := strings.Split(*access, ",")

	log.Info().Msgf("TPS to send: %v", *tps)

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

	// RLP decode the key
	ServiceAccountPrivateKey, err := flow.DecodeAccountPrivateKey(serviceAccountPrivateKeyBytes)
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

	flowClient, err := client.New(accessNodeAddrs[0], grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to initialize Flow client")
	}

	lg, err := utils.NewContLoadGenerator(
		log,
		flowClient,
		accessNodeAddrs[0],
		priv,
		&serviceAccountAddress,
		&fungibleTokenAddress,
		&flowTokenAddress,
		*tps,
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to create new cont load generator")
	}

	err = lg.Init()
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to init loader")
	}

	lg.Start()

	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}
