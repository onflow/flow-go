package execution

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/examples"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	FungibleTokenContractsBaseURL = "https://raw.githubusercontent.com/onflow/flow-ft/master/src/contracts/"

	CustodialDeposit = "CustodialDeposit.cdc"
	FlowToken        = "FlowToken.cdc"
	FungibleToken    = "FungibleToken.cdc"
	TokenForwarding  = "TokenForwarding.cdc"
)

const (
	// More transactions listed here: https://github.com/onflow/flow-ft/tree/master/transactions
	FungibleTokenTransactionsBaseURL = "https://raw.githubusercontent.com/onflow/flow-ft/master/src/transactions/"

	SetupAccount   = "setup_account.cdc"
	MintTokens     = "mint_tokens.cdc"
	GetSupply      = "get_supply.cdc"
	GetBalance     = "get_balance.cdc"
	TransferTokens = "transfer_tokens.cdc"
)

const (
	ResultFile = "/tmp/tx_per_second_test_%s.txt"
)

var (
	fungibleTokenAddress flowsdk.Address
	flowTokenAddress     flowsdk.Address
)

func TestTransactionsPerSecondBenchmark(t *testing.T) {
	suite.Run(t, new(TransactionsPerSecondSuite))
}

type TransactionsPerSecondSuite struct {
	Suite
	ref          *flowsdk.BlockHeader
	accounts     map[flowsdk.Address]*flowsdk.AccountKey
	privateKeys  map[string][]byte
	signers      map[flowsdk.Address]crypto.InMemorySigner
	rootAcctAddr flowsdk.Address
	rootAcctKey  *flowsdk.AccountKey
	rootSigner   crypto.Signer
}

func (gs *TransactionsPerSecondSuite) TestTransactionsPerSecond() {
	gs.SetTokenAddresses()
	gs.accounts = map[flowsdk.Address]*flowsdk.AccountKey{}
	gs.privateKeys = map[string][]byte{}
	gs.signers = map[flowsdk.Address]crypto.InMemorySigner{}

	flowClient, err := client.New(fmt.Sprintf(":%s", gs.net.AccessPorts[testnet.AccessNodeAPIPort]), grpc.WithInsecure())
	require.NoError(gs.T(), err, "could not get client")

	gs.rootAcctAddr, gs.rootAcctKey, gs.rootSigner = RootAccountWithKey(flowClient, unittest.ServiceAccountPrivateKeyHexSDK)

	// wait for first finalized block, called blockA
	// blockA := gs.BlockState.WaitForFirstFinalized(gs.T())
	// gs.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())
	finalizedBlock, err := flowClient.GetLatestBlockHeader(context.Background(), false)
	require.NoError(gs.T(), err, "could not get client")

	gs.ref = finalizedBlock

	//gs.DeployFungibleAndFlowTokens(flowClient) // not needed since flow contract is now deployed by default

	// createAccountWG := sync.WaitGroup{}
	for i := 0; i < 50; i++ {
		finalizedBlock, err := flowClient.GetLatestBlockHeader(context.Background(), false)
		gs.ref = finalizedBlock
		examples.Handle(err)
		addr, key := gs.CreateAccountAndTransfer(flowClient)
		gs.accounts[addr] = key
	}

	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/metrics", gs.net.AccessPorts[testnet.ExeNodeMetricsPort]))
	require.NoError(gs.T(), err, "could not get metrics")
	startNum := 0
	startTime := time.Now()
	// body, err := ioutil.ReadAll(resp.Body)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "execution_runtime_total_executed_transactions") {
			startNum, err = strconv.Atoi(strings.Split(line, " ")[1])
			require.NoError(gs.T(), err, "could not get metrics")
		}
	}
	err = scanner.Err()
	require.NoError(gs.T(), err, "could not get metrics")
	resp.Body.Close()

	fmt.Println("==========START", startNum, startTime)

	// for i := 0; i < 10; i++ {
	fmt.Println("Transfering tokens")
	rounds := 10
	transferWG := sync.WaitGroup{}
	prevAddr := flowTokenAddress
	finalizedBlock, err = flowClient.GetLatestBlockHeader(context.Background(), false)
	gs.ref = finalizedBlock
	examples.Handle(err)

	for accountAddr, accountKey := range gs.accounts {
		transferWG.Add(rounds)
		go func(fromAddr, toAddr flowsdk.Address, accKey *flowsdk.AccountKey) {
			for i := 0; i < rounds; i++ {
				gs.Transfer10Tokens(flowClient, fromAddr, toAddr, accKey)
				transferWG.Done()
			}
		}(accountAddr, prevAddr, accountKey)
		prevAddr = accountAddr
	}

	go func() {
		time.Sleep(60 * time.Second)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/metrics", gs.net.AccessPorts[testnet.ExeNodeMetricsPort]))
		require.NoError(gs.T(), err, "could not get metrics")
		endNum := 0
		endTime := time.Now()
		defer resp.Body.Close()
		// body, err := ioutil.ReadAll(resp.Body)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "execution_runtime_total_executed_transactions") {
				endNum, err = strconv.Atoi(strings.Split(line, " ")[1])
				require.NoError(gs.T(), err, "could not get metrics")
			}
		}
		err = scanner.Err()
		require.NoError(gs.T(), err, "could not get metrics")

		fmt.Println("==========END", endNum, endTime)

		dur := endTime.Sub(startTime)
		tps := float64(endNum-startNum) / dur.Seconds()

		fmt.Println("==========TPS", tps)
		err = logTPSToFile(tps)
		require.NoErrorf(gs.T(), err, "failed to write tps to file")
	}()

	transferWG.Wait()
	// }

	gs.T().FailNow()

}

func (gs *TransactionsPerSecondSuite) SetTokenAddresses() {
	addressGen := flowsdk.NewAddressGenerator(flowsdk.Testnet)
	_, err := addressGen.NextAddress()
	examples.Handle(err)
	fungibleTokenAddress, err = addressGen.NextAddress()
	examples.Handle(err)
	flowTokenAddress, err = addressGen.NextAddress()
	examples.Handle(err)
}

func (gs *TransactionsPerSecondSuite) DeployFungibleAndFlowTokens(flowClient *client.Client) {
	ctx := context.Background()
	// Deploy the FT contract
	ftCode, err := DownloadFile(FungibleTokenContractsBaseURL + FungibleToken)
	require.NoError(gs.T(), err)
	deployFTScript, err := templates.CreateAccount(nil, ftCode)
	require.NoError(gs.T(), err)

	deployContractTx := flowsdk.NewTransaction().
		SetReferenceBlockID(gs.ref.ID).
		AddAuthorizer(gs.rootAcctAddr).
		SetScript(deployFTScript).
		SetProposalKey(gs.rootAcctAddr, gs.rootAcctKey.ID, gs.rootAcctKey.SequenceNumber).
		SetPayer(gs.rootAcctAddr)

	err = deployContractTx.SignEnvelope(
		gs.rootAcctAddr,
		gs.rootAcctKey.ID,
		gs.rootSigner,
	)
	require.NoError(gs.T(), err)

	err = flowClient.SendTransaction(ctx, *deployContractTx)
	require.NoError(gs.T(), err)

	deployContractTxResp := WaitForFinalized(ctx, flowClient, deployContractTx.ID())
	require.NoError(gs.T(), deployContractTxResp.Error)

	// Successful Tx, increment sequence number
	gs.rootAcctKey.SequenceNumber++

	for _, event := range deployContractTxResp.Events {
		fmt.Printf("EVENT %+v\n", event)
		fmt.Println(event.ID())
		fmt.Println(event.Type)
		fmt.Println(event.Value)
		if event.Type == flowsdk.EventAccountCreated {
			accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
			fungibleTokenAddress = accountCreatedEvent.Address()
		}
	}

	fmt.Println("FT Address:", fungibleTokenAddress.Hex())

	// Deploy the Flow Token contract
	flowTokenCodeRaw, err := DownloadFile(FungibleTokenContractsBaseURL + FlowToken)
	require.NoError(gs.T(), err)
	flowTokenCode := strings.ReplaceAll(string(flowTokenCodeRaw), "0x02", "0x"+fungibleTokenAddress.Hex())

	// Use the same root account key for simplicity
	deployFlowTokenScript, err := templates.CreateAccount([]*flowsdk.AccountKey{gs.rootAcctKey}, []byte(flowTokenCode))
	require.NoError(gs.T(), err)

	deployFlowTokenContractTx := flowsdk.NewTransaction().
		SetReferenceBlockID(gs.ref.ID).
		AddAuthorizer(gs.rootAcctAddr).
		SetScript(deployFlowTokenScript).
		SetProposalKey(gs.rootAcctAddr, gs.rootAcctKey.ID, gs.rootAcctKey.SequenceNumber).
		SetPayer(gs.rootAcctAddr)

	err = deployFlowTokenContractTx.SignEnvelope(
		gs.rootAcctAddr,
		gs.rootAcctKey.ID,
		gs.rootSigner,
	)
	require.NoError(gs.T(), err)

	err = flowClient.SendTransaction(ctx, *deployFlowTokenContractTx)
	require.NoError(gs.T(), err)

	deployFlowTokenContractTxResp := WaitForFinalized(ctx, flowClient, deployFlowTokenContractTx.ID())
	require.NoError(gs.T(), deployFlowTokenContractTxResp.Error)

	// Successful Tx, increment sequence number
	gs.rootAcctKey.SequenceNumber++

	for _, event := range deployFlowTokenContractTxResp.Events {
		fmt.Printf("%+v\n", event)

		if event.Type == flowsdk.EventAccountCreated {
			accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
			flowTokenAddress = accountCreatedEvent.Address()
		}
	}

	fmt.Println("Flow Token Address:", flowTokenAddress.Hex())
}

func (gs *TransactionsPerSecondSuite) CreateAccountAndTransfer(flowClient *client.Client) (flowsdk.Address, *flowsdk.AccountKey) {
	ctx := context.Background()

	myPrivateKey := examples.RandomPrivateKey()
	accountKey := flowsdk.NewAccountKey().
		FromPrivateKey(myPrivateKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flowsdk.AccountKeyWeightThreshold)
	mySigner := crypto.NewInMemorySigner(myPrivateKey, accountKey.HashAlgo)

	// Generate an account creation script
	createAccountScript, err := templates.CreateAccount([]*flowsdk.AccountKey{accountKey}, nil)
	examples.Handle(err)

	createAccountTx := flowsdk.NewTransaction().
		SetReferenceBlockID(gs.ref.ID).
		AddAuthorizer(gs.rootAcctAddr).
		SetScript(createAccountScript).
		SetProposalKey(gs.rootAcctAddr, gs.rootAcctKey.ID, gs.rootAcctKey.SequenceNumber).
		SetPayer(gs.rootAcctAddr)

	err = createAccountTx.SignEnvelope(gs.rootAcctAddr, gs.rootAcctKey.ID, gs.rootSigner)
	examples.Handle(err)

	err = flowClient.SendTransaction(ctx, *createAccountTx)
	examples.Handle(err)

	accountCreationTxRes := WaitForFinalized(ctx, flowClient, createAccountTx.ID())
	examples.Handle(accountCreationTxRes.Error)

	// Successful Tx, increment sequence number
	gs.rootAcctKey.SequenceNumber++
	accountAddress := flowsdk.Address{}
	if len(accountCreationTxRes.Events) == 0 {
		accountCreationTxRes = examples.WaitForSeal(ctx, flowClient, createAccountTx.ID())
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

	// Setup the account
	accountSetupScript := GenerateSetupAccountScript(fungibleTokenAddress, flowTokenAddress)

	accountSetupTx := flowsdk.NewTransaction().
		SetReferenceBlockID(gs.ref.ID).
		SetScript(accountSetupScript).
		SetProposalKey(accountAddress, accountKey.ID, accountKey.SequenceNumber).
		SetPayer(accountAddress).
		AddAuthorizer(accountAddress)

	err = accountSetupTx.SignEnvelope(accountAddress, accountKey.ID, mySigner)
	examples.Handle(err)

	err = flowClient.SendTransaction(ctx, *accountSetupTx)
	examples.Handle(err)

	accountSetupTxResp := WaitForFinalized(ctx, flowClient, accountSetupTx.ID())
	examples.Handle(accountSetupTxResp.Error)

	// Successful Tx, increment sequence number
	accountKey.SequenceNumber++

	// Mint to the new account
	flowTokenAcc, err := flowClient.GetAccount(context.Background(), flowTokenAddress)
	examples.Handle(err)
	flowTokenAccKey := flowTokenAcc.Keys[0]

	// Mint 10 tokens
	mintScript := GenerateMintScript(fungibleTokenAddress, flowTokenAddress, accountAddress)
	mintTx := flowsdk.NewTransaction().
		SetReferenceBlockID(gs.ref.ID).
		SetScript(mintScript).
		SetProposalKey(accountAddress, accountKey.ID, accountKey.SequenceNumber).
		SetPayer(accountAddress).
		AddAuthorizer(flowTokenAddress)

	err = mintTx.SignPayload(flowTokenAddress, flowTokenAccKey.ID, gs.rootSigner)
	examples.Handle(err)

	err = mintTx.SignEnvelope(accountAddress, accountKey.ID, mySigner)
	examples.Handle(err)

	err = flowClient.SendTransaction(ctx, *mintTx)
	examples.Handle(err)

	mintTxResp := WaitForFinalized(ctx, flowClient, mintTx.ID())
	examples.Handle(mintTxResp.Error)

	// Successful Tx, increment sequence number
	accountKey.SequenceNumber++
	return accountAddress, accountKey
}

func (gs *TransactionsPerSecondSuite) Transfer10Tokens(flowClient *client.Client, fromAddr, toAddr flowsdk.Address, fromKey *flowsdk.AccountKey) {
	ctx := context.Background()

	// Transfer 10 tokens
	transferScript := GenerateTransferScript(fungibleTokenAddress, flowTokenAddress, toAddr)
	transferTx := flowsdk.NewTransaction().
		SetReferenceBlockID(gs.ref.ID).
		SetScript(transferScript).
		SetProposalKey(fromAddr, fromKey.ID, fromKey.SequenceNumber).
		SetPayer(fromAddr).
		AddAuthorizer(fromAddr)

	// err = transferTx.SignPayload(flowTokenAddress, flowTokenAccKey.ID, rootSigner)
	// examples.Handle(err)

	err := transferTx.SignEnvelope(fromAddr, fromKey.ID, gs.signers[fromAddr])
	examples.Handle(err)

	err = flowClient.SendTransaction(ctx, *transferTx)
	examples.Handle(err)

	transferTxResp := WaitForFinalized(ctx, flowClient, transferTx.ID())

	// Successful Tx, increment sequence number
	fromKey.SequenceNumber++

	if transferTxResp.Error != nil {
		fmt.Println(transferTxResp.Error)
		// Do not fail, so that we can continue loop
		return
	}
}

// DownloadFile will download a url a byte slice
func DownloadFile(url string) ([]byte, error) {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

const defaultRootKeySeed = "elephant ears space cowboy octopus rodeo potato cannon pineapple"

func RootAccountWithKey(flowClient *client.Client, key string) (flowsdk.Address, *flowsdk.AccountKey, crypto.Signer) {
	addr := flowsdk.ServiceAddress(flowsdk.Mainnet)

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
	// Handle(err)

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
		// Handle(err)
	}

	fmt.Println()
	fmt.Printf("Transaction %s finalized\n", id)

	return result
}

func GenerateSetupAccountScript(ftAddr, flowToken flowsdk.Address) []byte {
	setupCode, err := DownloadFile(FungibleTokenTransactionsBaseURL + SetupAccount)
	examples.Handle(err)

	withFTAddr := strings.ReplaceAll(string(setupCode), "0x02", "0x"+ftAddr.Hex())
	withFlowTokenAddr := strings.ReplaceAll(string(withFTAddr), "0x03", "0x"+flowToken.Hex())

	return []byte(withFlowTokenAddr)
}

// GenerateMintScript Creates a script that mints an 10 FTs
func GenerateMintScript(ftAddr, flowToken, toAddr flowsdk.Address) []byte {
	mintCode, err := DownloadFile(FungibleTokenTransactionsBaseURL + MintTokens)
	examples.Handle(err)

	withFTAddr := strings.ReplaceAll(string(mintCode), "0x02", "0x"+ftAddr.Hex())
	withFlowTokenAddr := strings.Replace(string(withFTAddr), "0x03", "0x"+flowToken.Hex(), 1)
	withToAddr := strings.Replace(string(withFlowTokenAddr), "0x03", "0x"+toAddr.Hex(), 1)

	withAmount := strings.Replace(string(withToAddr), "10.0", "1.0", 1)

	return []byte(withAmount)
}

// GenerateTransferScript Creates a script that mints an 10 FTs
func GenerateTransferScript(ftAddr, flowToken, toAddr flowsdk.Address) []byte {
	mintCode, err := DownloadFile(FungibleTokenTransactionsBaseURL + TransferTokens)
	examples.Handle(err)

	withFTAddr := strings.ReplaceAll(string(mintCode), "0x02", "0x"+ftAddr.Hex())
	withFlowTokenAddr := strings.Replace(string(withFTAddr), "0x03", "0x"+flowToken.Hex(), 1)
	withToAddr := strings.Replace(string(withFlowTokenAddr), "0x04", "0x"+toAddr.Hex(), 1)

	withAmount := strings.Replace(string(withToAddr), "10.0", "0.01", 1)

	return []byte(withAmount)
}

func logTPSToFile(tps float64) error {
	resultFileName := fmt.Sprintf(ResultFile, time.Now().Format("2006_01_02_15_04_05"))
	tpsStr := fmt.Sprintf("%f\n", tps)
	return ioutil.WriteFile(resultFileName, []byte(tpsStr), 0644)
}
