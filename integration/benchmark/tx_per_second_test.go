package execution

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/integration/tests/execution"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// Pinned to specific commit
	// More transactions listed here: https://github.com/onflow/flow-ft/tree/0e8024a483ce85c06eb165c2d4c9a5795ba167a1/transactions
	FungibleTokenTransactionsBaseURL = "https://raw.githubusercontent.com/onflow/flow-ft/0e8024a483ce85c06eb165c2d4c9a5795ba167a1/src/transactions/"
	TransferTokens                   = "transfer_tokens.cdc"
)

// Output file which records the transaction per second for a test run
const (
	ResultFile = "tx_per_second_test_%s.txt"
)

const (
	// total test accounts to create
	// This is a long running test. On a local environment use more conservative numbers for TotalAccounts (~3)
	TotalAccounts = 20
	// each account transfers 10 tokens to the next account RoundsOfTransfer number of times
	RoundsOfTransfer = 50
	// threshold for TPS
	TPSThreshold = float64(1)
)

var (
	fungibleTokenAddress flowsdk.Address
	flowTokenAddress     flowsdk.Address

	fileCache = map[string][]byte{}

	tmpDir string
)

// TestTransactionsPerSecondBenchmark measures the average number of transactions executed per second by an execution node
func TestTransactionsPerSecondBenchmark(t *testing.T) {
	suite.Run(t, new(TransactionsPerSecondSuite))
}

type TransactionsPerSecondSuite struct {
	execution.Suite
	flowClient      *client.Client
	ref             *flowsdk.BlockHeader
	accounts        map[flowsdk.Address]*flowsdk.AccountKey
	privateKeys     map[string][]byte
	signers         map[flowsdk.Address]crypto.InMemorySigner
	rootAcctAddr    flowsdk.Address
	rootAcctKey     *flowsdk.AccountKey
	rootSigner      crypto.Signer
	rootSignerLock  sync.Mutex
	sequenceNumbers []uint64
	accessAddr      string
	privateKeyHex   string
	metricsAddr     string
}

func (gs *TransactionsPerSecondSuite) SetupTest() {
	// this sets up the testing network. no need to run if testing against a non-local network
	gs.Suite.SetupTest()

	// Change these to corresponding values if using against non-local testnet
	gs.accessAddr = fmt.Sprintf(":%s", gs.AccessPort())
	gs.privateKeyHex = gs.privateKey()
	gs.metricsAddr = fmt.Sprintf("http://localhost:%s/metrics", gs.MetricsPort())

	// Get tmp dir
	tmpDir = os.Getenv("TMP")
	if len(tmpDir) == 0 {
		tmpDir = "/tmp"
	}

	// Cache file
	_, err := DownloadFile(FungibleTokenTransactionsBaseURL + TransferTokens)
	handle(err)
}

func (gs *TransactionsPerSecondSuite) privateKey() string {
	serviceAccountPrivateKeyBytes, err := hex.DecodeString(unittest.ServiceAccountPrivateKeyHex)
	if err != nil {
		panic("error while hex decoding hardcoded root key")
	}

	// RLP decode the key
	ServiceAccountPrivateKey, err := flow.DecodeAccountPrivateKey(serviceAccountPrivateKeyBytes)
	if err != nil {
		panic("error while decoding hardcoded root key bytes")
	}

	// get the private key string
	return hex.EncodeToString(ServiceAccountPrivateKey.PrivateKey.Encode())
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
	gs.SetTokenAddresses()

	// Initialize some book keeping
	gs.accounts = map[flowsdk.Address]*flowsdk.AccountKey{}
	gs.privateKeys = map[string][]byte{}
	gs.signers = map[flowsdk.Address]crypto.InMemorySigner{}

	// Setup the client, not using the suite to generate client since we may want to call external testnets
	flowClient, err := client.New(gs.accessAddr, grpc.WithInsecure())
	require.NoError(gs.T(), err, "could not get client")
	gs.flowClient = flowClient

	// Grab the service account info
	gs.rootAcctAddr, gs.rootAcctKey, gs.rootSigner = ServiceAccountWithKey(flowClient, gs.privateKeyHex)

	// Set last finalized block to be used as ref for transactions
	finalizedBlock, err := flowClient.GetLatestBlockHeader(context.Background(), false)
	require.NoError(gs.T(), err, "could not update finalized block")
	gs.ref = finalizedBlock

	gs.AddKeys(flowClient)

	// flow token is deployed by default, can just start creating test accounts
	accountsWG := sync.WaitGroup{}

	for i := 1; i < TotalAccounts; i++ {
		// Refresh finalized block we're using as reference
		// finalizedBlock, err := flowClient.GetLatestBlockHeader(context.Background(), false)
		// gs.ref = finalizedBlock
		// examples.Handle(err)
		accountsWG.Add(1)
		go func(keyIndex int) {
			// Create an account and transfer some funds to it.
			addr, key := gs.CreateAccountAndTransfer(keyIndex)
			gs.accounts[addr] = key
			accountsWG.Done()
		}(i)

	}

	accountsWG.Wait()

	// Transferring Tokens
	transferWG := sync.WaitGroup{}
	prevAddr := flowTokenAddress
	gs.ref, err = flowClient.GetLatestBlockHeader(context.Background(), false)
	handle(err)

	for accountAddr, accountKey := range gs.accounts {
		if prevAddr != flowTokenAddress {
			transferWG.Add(RoundsOfTransfer)
			go func(fromAddr, toAddr flowsdk.Address, accKey *flowsdk.AccountKey) {
				for i := 0; i < RoundsOfTransfer; i++ {
					gs.Transfer10Tokens(flowClient, fromAddr, toAddr, accKey)
					transferWG.Done()
				}
			}(accountAddr, prevAddr, accountKey)
		}
		prevAddr = accountAddr
	}

	// Record the TPS
	resultFileName := fmt.Sprintf(ResultFile, time.Now().Format("2006_01_02_15_04_05"))
	done := make(chan struct{})
	go gs.sampleTotalExecutedTransactionMetric(resultFileName, done) // kick of the sampler

	// wait for all transfer rounds to finish
	transferWG.Wait()

	close(done) // stop the sampler
	require.FileExists(gs.T(), filepath.Join(tmpDir, resultFileName), "did not log TPS to file, may need to increase timeout")
}

// SetTokenAddresses sets the addresses for the Fungible token and the Flow token contract that were bootstrapped as part of genesis state
// The function assumes that Fungible token contract and the Flow token contract are both deployed during execution node bootstrap
// and since address generation is deterministic, the second and the third address are assumed to be that of Fungible
// Token and Flow token
func (gs *TransactionsPerSecondSuite) SetTokenAddresses() {
	addressGen := flowsdk.NewAddressGenerator(flowsdk.Testnet)
	fmt.Println("Root Service Address:", addressGen.NextAddress())
	fungibleTokenAddress = addressGen.NextAddress()
	fmt.Println("Fungible Address:", fungibleTokenAddress)
	flowTokenAddress = addressGen.NextAddress()
	fmt.Println("Flow Address:", flowTokenAddress)
}

// CreateAccountAndTransfer will create an account and transfer 1000 tokens to it
func (gs *TransactionsPerSecondSuite) CreateAccountAndTransfer(keyIndex int) (flowsdk.Address, *flowsdk.AccountKey) {
	ctx := context.Background()

	myPrivateKey := common.RandomPrivateKey()
	accountKey := flowsdk.NewAccountKey().
		FromPrivateKey(myPrivateKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flowsdk.AccountKeyWeightThreshold)
	mySigner := crypto.NewInMemorySigner(myPrivateKey, accountKey.HashAlgo)

	// Generate the account creation transaction
	createAccountTx := templates.CreateAccount([]*flowsdk.AccountKey{accountKey}, nil, gs.rootAcctAddr).
		SetReferenceBlockID(gs.ref.ID).
		SetProposalKey(gs.rootAcctAddr, keyIndex, gs.sequenceNumbers[keyIndex]).
		SetPayer(gs.rootAcctAddr)

	gs.rootSignerLock.Lock()
	err := createAccountTx.SignEnvelope(gs.rootAcctAddr, keyIndex, gs.rootSigner)
	handle(err)

	gs.ref, err = gs.flowClient.GetLatestBlockHeader(context.Background(), false)
	handle(err)

	// execute the CreateAccount transaction
	err = gs.flowClient.SendTransaction(ctx, *createAccountTx)
	handle(err)

	createAccountTxID := createAccountTx.ID()

	gs.rootSignerLock.Unlock()

	accountCreationTxRes := WaitForFinalized(ctx, gs.flowClient, createAccountTxID)
	handle(accountCreationTxRes.Error)

	// Successful Tx, increment sequence number
	gs.sequenceNumbers[keyIndex]++
	accountAddress := flowsdk.Address{}
	for len(accountCreationTxRes.Events) == 0 {
		accountCreationTxRes = WaitForFinalized(ctx, gs.flowClient, createAccountTxID)
	}
	for _, event := range accountCreationTxRes.Events {
		fmt.Println(event)

		if event.Type == flowsdk.EventAccountCreated {
			accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
			accountAddress = accountCreatedEvent.Address()
		}
	}

	fmt.Println("My Address:", accountAddress.Hex())

	// Save key and signer
	gs.signers[accountAddress] = mySigner
	gs.privateKeys[accountAddress.String()] = myPrivateKey.Encode()

	// Transfer 1000 tokens
	transferScript := GenerateTransferScript(fungibleTokenAddress, flowTokenAddress, accountAddress, 1000)
	transferTx := flowsdk.NewTransaction().
		SetReferenceBlockID(gs.ref.ID).
		SetScript(transferScript).
		SetProposalKey(gs.rootAcctAddr, keyIndex, gs.sequenceNumbers[keyIndex]).
		SetPayer(gs.rootAcctAddr).
		AddAuthorizer(gs.rootAcctAddr)

	gs.rootSignerLock.Lock()
	err = transferTx.SignEnvelope(gs.rootAcctAddr, keyIndex, gs.rootSigner)
	handle(err)

	gs.ref, err = gs.flowClient.GetLatestBlockHeader(context.Background(), false)
	handle(err)

	err = gs.flowClient.SendTransaction(ctx, *transferTx)
	handle(err)

	transferTxID := transferTx.ID()
	gs.rootSignerLock.Unlock()

	transferTxResp := WaitForFinalized(ctx, gs.flowClient, transferTxID)
	handle(transferTxResp.Error)

	// Successful Tx, increment sequence number
	gs.sequenceNumbers[keyIndex]++
	return accountAddress, accountKey
}

// Transfer10Tokens transfers 10 tokens
func (gs *TransactionsPerSecondSuite) Transfer10Tokens(flowClient *client.Client, fromAddr, toAddr flowsdk.Address, fromKey *flowsdk.AccountKey) {
	ctx := context.Background()

	// Transfer 10 tokens
	transferScript := GenerateTransferScript(fungibleTokenAddress, flowTokenAddress, toAddr, 10)
	transferTx := flowsdk.NewTransaction().
		SetReferenceBlockID(gs.ref.ID).
		SetScript(transferScript).
		SetProposalKey(fromAddr, fromKey.Index, fromKey.SequenceNumber).
		SetPayer(fromAddr).
		AddAuthorizer(fromAddr)

	err := transferTx.SignEnvelope(fromAddr, fromKey.Index, gs.signers[fromAddr])
	handle(err)

	err = flowClient.SendTransaction(ctx, *transferTx)
	handle(err)

	transferTxResp := WaitForFinalized(ctx, flowClient, transferTx.ID())

	// Successful Tx, increment sequence number
	fromKey.SequenceNumber++

	if transferTxResp.Error != nil {
		fmt.Println(transferTxResp.Error)
		// Do not fail, so that we can continue loop
		return
	}
}

// logTPSToFile records the instantaneous as average values to the output file
func logTPSToFile(msg string, resultFileName string) error {
	fullResultFilePath := filepath.Join(tmpDir, resultFileName)

	resultFile, err := os.OpenFile(fullResultFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer resultFile.Close()
	if _, err := resultFile.WriteString(msg + "\n"); err != nil {
		return err
	}
	return nil
}

// TODO: Consider moving some of the following helpers to a common package, or just use any that are in the SDK once they're added there

// DownloadFile will download a url a byte slice
func DownloadFile(url string) ([]byte, error) {
	if file, ok := fileCache[url]; ok {
		return file, nil
	}
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	fileCache[url], err = ioutil.ReadAll(resp.Body)
	return fileCache[url], err
}

func ServiceAccountWithKey(flowClient *client.Client, key string) (flowsdk.Address, *flowsdk.AccountKey, crypto.Signer) {
	addr := flowsdk.ServiceAddress(flowsdk.Testnet)

	acc, err := flowClient.GetAccount(context.Background(), addr)
	if err != nil {
		panic(err)
	}

	accountKey := acc.Keys[0]

	privateKey, err := crypto.DecodePrivateKeyHex(accountKey.SigAlgo, key)
	if err != nil {
		panic(err)
	}

	signer := crypto.NewInMemorySigner(privateKey, accountKey.HashAlgo)

	return addr, accountKey, signer
}

func WaitForFinalized(ctx context.Context, c *client.Client, id flowsdk.Identifier) *flowsdk.TransactionResult {
	result, err := c.GetTransactionResult(ctx, id)
	handle(err)

	fmt.Printf("Waiting for transaction %s to be finalized...\n", id)
	errCount := 0
	for result == nil || (result.Status != flowsdk.TransactionStatusFinalized && result.Status != flowsdk.TransactionStatusSealed) {
		time.Sleep(time.Second)
		result, err = c.GetTransactionResult(ctx, id)
		if err != nil {
			fmt.Print("x")
			errCount++
			if errCount >= 10 {
				return &flowsdk.TransactionResult{
					Error: err,
				}
			}
		} else {
			fmt.Print(".")
		}
	}

	fmt.Println()
	fmt.Printf("Transaction %s finalized\n", id)

	return result
}

// GenerateTransferScript Creates a script that transfer some amount of FTs
func GenerateTransferScript(ftAddr, flowToken, toAddr flowsdk.Address, amount int) []byte {
	mintCode, err := DownloadFile(FungibleTokenTransactionsBaseURL + TransferTokens)
	handle(err)

	withFTAddr := strings.ReplaceAll(string(mintCode), "0x02", "0x"+ftAddr.Hex())
	withFlowTokenAddr := strings.Replace(string(withFTAddr), "0x03", "0x"+flowToken.Hex(), 1)
	withToAddr := strings.Replace(string(withFlowTokenAddr), "0x04", "0x"+toAddr.Hex(), 1)

	withAmount := strings.Replace(string(withToAddr), fmt.Sprintf("%d.0", amount), "0.01", 1)

	return []byte(withAmount)
}

func bytesToCadenceArray(b []byte) cadence.Array {
	values := make([]cadence.Value, len(b))

	for i, v := range b {
		values[i] = cadence.NewUInt8(v)
	}

	return cadence.NewArray(values)
}

func (gs *TransactionsPerSecondSuite) AddKeys(flowClient *client.Client) {
	ctx := context.Background()

	gs.sequenceNumbers = make([]uint64, TotalAccounts)
	publicKeysStr := strings.Builder{}
	accountKeyBytes := gs.rootAcctKey.Encode()

	for i := 0; i < TotalAccounts; i++ {
		publicKeysStr.WriteString("signer.addPublicKey(publicKey)\n")
	}
	script := fmt.Sprintf(`
	transaction(publicKey: [UInt8]) {
		prepare(signer: AuthAccount) {
			%s
		}
	}
`, publicKeysStr.String())

	addKeysTx := flowsdk.NewTransaction().
		SetReferenceBlockID(gs.ref.ID).
		SetScript([]byte(script)).
		SetProposalKey(gs.rootAcctAddr, gs.rootAcctKey.Index, gs.rootAcctKey.SequenceNumber).
		SetPayer(gs.rootAcctAddr).
		AddAuthorizer(gs.rootAcctAddr)

	err := addKeysTx.AddArgument(bytesToCadenceArray(accountKeyBytes))
	handle(err)

	err = addKeysTx.SignEnvelope(gs.rootAcctAddr, gs.rootAcctKey.Index, gs.rootSigner)
	handle(err)

	err = flowClient.SendTransaction(ctx, *addKeysTx)
	handle(err)

	addKeysTxResp := WaitForFinalized(ctx, flowClient, addKeysTx.ID())
	handle(addKeysTxResp.Error)

	// Successful Tx, increment sequence number
	gs.rootAcctKey.SequenceNumber++

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

	logAvg := func() {
		total := 0.0
		for _, inst := range instantaneous {
			total = total + inst
		}
		avg := total / (float64(len(instantaneous)))

		fmt.Printf("Average TPS ===========> : %f\n", avg)
		logTPSToFile(fmt.Sprintf("average TPS %f", avg), resultFileName)

		fmt.Printf("Total transactions ===========> : %d\n", totalExecutedTx)
		logTPSToFile(fmt.Sprintf("total transactions %d", totalExecutedTx), resultFileName)

		require.Greater(gs.T(), avg, TPSThreshold)
		if os.Getenv("ENV") == "TEAMCITY" {
			logTPSToFile(fmt.Sprintf("##teamcity[buildStatisticValue key='BenchmarkTPS' value='%f']", avg), fmt.Sprintf(ResultFile, "teamcity"))
		}
	}

	// sample every 1 minute
	minTicker := time.NewTicker(sampleTime)
	startTime := time.Now()
	totalExecutedTx = sample() // baseline the executed tx count

	for {
		select {
		case <-done:
			minTicker.Stop()
			logAvg()
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

			// Update finalized block so it doesn't get too stale
			gs.ref, err = gs.flowClient.GetLatestBlockHeader(context.Background(), false)
			require.NoError(gs.T(), err, "could not update finalized block")
		}
	}
}

func handle(err error) {
	if err != nil {
		panic(err)
	}
}
