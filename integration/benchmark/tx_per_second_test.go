package benchmark

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/integration/tests/execution"
	"github.com/dapperlabs/flow-go/integration/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// Pinned to specific commit
	// More transactions listed here: https://github.com/onflow/flow-ft/tree/0e8024a483ce85c06eb165c2d4c9a5795ba167a1/transactions
	FungibleTokenTransactionsBaseURL = "https://raw.githubusercontent.com/onflow/flow-ft/0e8024a483ce85c06eb165c2d4c9a5795ba167a1/src/transactions/"
	TransferTokens                   = "transfer_tokens.cdc"
)

// Output file which records the transaction per second for a test run
const (
	ResultFile = "/tmp/tx_per_second_test_%s.txt"
)

const (
	// total test accounts to create
	// This is a long running test. On a local environment use more conservative numbers for TotalAccounts (~3)
	TotalAccounts = 10
	// each account transfers 10 tokens to the next account RoundsOfTransfer number of times
	RoundsOfTransfer = 50
)

var (
	fungibleTokenAddress flowsdk.Address
	flowTokenAddress     flowsdk.Address
)

// TestTransactionsPerSecondBenchmark measures the average number of transactions executed per second by an execution node
func TestTransactionsPerSecondBenchmark(t *testing.T) {
	suite.Run(t, new(TransactionsPerSecondSuite))
}

type TransactionsPerSecondSuite struct {
	execution.Suite
	accessAddr  string
	metricsAddr string
}

func (gs *TransactionsPerSecondSuite) SetupTest() {
	// this sets up the testing network. no need to run if testing against a non-local network
	gs.Suite.SetupTest()

	// Change these to corresponding values if using against non-local testnet
	gs.accessAddr = fmt.Sprintf(":%s", gs.AccessPort())
	gs.metricsAddr = fmt.Sprintf("http://localhost:%s/metrics", gs.MetricsPort())
}

// TestTransactionsPerSecond submits transactions and measures the average transaction per second of the execution node
// It does following:
// 1. Create new TotalAccounts number of accounts and transfers token to each account
// 2. Transfers 10 tokens from one account the the other repeatedly RoundsOfTransfer times.
// 3. While executing step 2, it samples the total execution metric from the execution node every 1 min
// 4. Prints the instantaneous and the average transaction per second values to an output file
func (gs *TransactionsPerSecondSuite) TestTransactionsPerSecond() {
	// Addresses are deterministic, so we should know what the FT interface and Flow Token contract addresses are
	// therefore lets just set them here
	chainID := flowsdk.Testnet
	accessNodeAddress := gs.accessAddr
	serviceAccountPrivateKeyHex := unittest.ServiceAccountPrivateKeyHex

	addressGen := flowsdk.NewAddressGenerator(chainID)
	serviceAccountAddress := addressGen.NextAddress()
	fmt.Println("Root Service Address:", serviceAccountAddress)
	fungibleTokenAddress := addressGen.NextAddress()
	fmt.Println("Fungible Address:", fungibleTokenAddress)
	flowTokenAddress := addressGen.NextAddress()
	fmt.Println("Flow Address:", flowTokenAddress)

	serviceAccountPrivateKeyBytes, err := hex.DecodeString(serviceAccountPrivateKeyHex)
	if err != nil {
		panic("error while hex decoding hardcoded root key")
	}

	// RLP decode the key
	ServiceAccountPrivateKey, err := flow.DecodeAccountPrivateKey(serviceAccountPrivateKeyBytes)
	if err != nil {
		panic("error while decoding hardcoded root key bytes")
	}

	// get the private key string
	priv := hex.EncodeToString(ServiceAccountPrivateKey.PrivateKey.Encode())

	flowClient, err := client.New(accessNodeAddress, grpc.WithInsecure())
	lg, err := utils.NewLoadGenerator(flowClient, priv, &serviceAccountAddress, &fungibleTokenAddress, &flowTokenAddress, 100, false)
	if err != nil {
		panic(err)
	}

	// Record the TPS
	resultFileName := fmt.Sprintf(ResultFile, time.Now().Format("2006_01_02_15_04_05"))
	done := make(chan struct{})
	go gs.sampleTotalExecutedTransactionMetric(resultFileName, done) // kick of the sampler

	rounds := 5
	// extra 3 is for setup
	for i := 0; i < rounds+3; i++ {
		lg.Next()
	}

	fmt.Println(lg.Stats())
	lg.Close()

	close(done) // stop the sampler
	require.FileExists(gs.T(), ResultFile, "did not log TPS to file, may need to increase timeout")
}

// logTPSToFile records the instantaneous as average values to the output file
func logTPSToFile(msg string, resultFileName string) error {
	resultFile, err := os.OpenFile(resultFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer resultFile.Close()
	if _, err := resultFile.WriteString(msg + "\n"); err != nil {
		return err
	}
	return nil
}

// Sample the ExecutedTransactionMetric Prometheus metric from the execution node every 1 minute
func (gs *TransactionsPerSecondSuite) sampleTotalExecutedTransactionMetric(resultFileName string, done chan struct{}) {
	fmt.Println("===== Starting metric sampler ======")
	defer fmt.Println("===== Stopping metric sampler ======")

	sampleTime := 1 * time.Minute // sampling frequency
	var instantaneous []float64   // a slice to store instantaneous values
	totalExecutedTx := 0          // cumulative count of total transactions

	sample := func() int {
		var txCount int
		resp, err := http.Get(gs.metricsAddr)
		require.NoError(gs.T(), err, "could not get metrics")
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "execution_runtime_total_executed_transactions") {
				txCount, err = strconv.Atoi(strings.Split(line, " ")[1])
				require.NoError(gs.T(), err, "could not get metrics")
				return txCount
			}
		}
		err = scanner.Err()
		require.Failf(gs.T(), "could not get metrics: %w", "%w", err)
		return 0
	}

	avg := func() {
		total := 0.0
		for _, inst := range instantaneous {
			total = total + inst
		}
		avg := total / (float64(len(instantaneous)))

		fmt.Printf("Average TPS ===========> : %f\n", avg)
		logTPSToFile(fmt.Sprintf("average TPS %f", avg), resultFileName)

		fmt.Printf("Total transactions ===========> : %d\n", totalExecutedTx)
		logTPSToFile(fmt.Sprintf("total transactions %d", totalExecutedTx), resultFileName)
	}

	// sample every 1 minute
	minTicker := time.NewTicker(sampleTime)
	startTime := time.Now()
	totalExecutedTx = sample() // baseline the executed tx count

	for {
		select {
		case <-done:
			minTicker.Stop()
			avg()
			return
		case <-minTicker.C:
			endTime := time.Now()
			newTotal := sample()
			dur := endTime.Sub(startTime)
			tps := float64(newTotal-totalExecutedTx) / dur.Seconds()

			startStr := startTime.Format("3:04:04 PM")
			fmt.Printf("TPS ===========> %s: %f\n", startStr, tps)

			instantaneous = append(instantaneous, tps)

			err := logTPSToFile(fmt.Sprintf("%s: %f", startStr, tps), resultFileName)
			require.NoErrorf(gs.T(), err, "failed to write instantaneous tps to file")

			// reset
			totalExecutedTx = newTotal
			startTime = endTime

			// // Update finalized block so it doesn't get too stale
			// gs.ref, err = gs.flowClient.GetLatestBlockHeader(context.Background(), false)
			// require.NoError(gs.T(), err, "could not update finalized block")
		}
	}
}
