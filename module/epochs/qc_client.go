package epochs

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// QCContractClient is a client to the QC Contract
type QCContractClient struct {
	accessAddress     string         // address of the access node
	qcContractAddress string         // QuorumCertificate contract address
	client            *client.Client // flow-go-sdk client to access node
	account           *sdk.Account
	accountKeyIndex   int
	privateKey        string
	signer            sdkcrypto.Signer
}

// NewQCContractClient returns a new client to the QC contract
func NewQCContractClient(privateKey, accountAddress string, accountKeyIndex int, accessAddress, qcContractAddress string) (*QCContractClient, error) {

	address := sdk.HexToAddress(accountAddress)

	// create a new instance of flow-go-sdk client
	flowClient, err := client.New(accessAddress, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not create flow client: %w", err)
	}

	// get account for given address
	account, err := flowClient.GetAccount(context.Background(), address)
	if err != nil {
		return nil, fmt.Errorf("could not get account: %w", err)
	}

	// check if account key index within range of keys
	if len(account.Keys) <= accountKeyIndex {
		return nil, fmt.Errorf("given account key index is bigger than the number of keys for this account")
	}

	// construct signer for signing transactions
	accountKey := account.Keys[accountKeyIndex]
	sk, err := sdkcrypto.DecodePrivateKeyHex(accountKey.SigAlgo, privateKey)
	if err != nil {
		return nil, fmt.Errorf("could not decode private key from hex: %v", err)
	}
	signer := sdkcrypto.NewInMemorySigner(sk, account.Keys[accountKeyIndex].HashAlgo)

	return &QCContractClient{
		accessAddress:     accessAddress,
		qcContractAddress: qcContractAddress,
		client:            flowClient,
		account:           account,
		accountKeyIndex:   accountKeyIndex,
		privateKey:        privateKey,
		signer:            signer,
	}, nil
}

// SubmitVote submits the given vote to the cluster QC aggregator smart
// contract. This function returns only once the transaction has been
// processed by the network. An error is returned if the transaction has
// failed and should be re-submitted.
func (c *QCContractClient) SubmitVote(ctx context.Context, vote *model.Vote) error {

	// get latest sealed block to execute transaction
	latestBlock, err := c.client.GetLatestBlock(ctx, true)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	// attach submit vote transaction template and build transaction
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSubmitVoteScript(c.getEnvironment())).
		SetGasLimit(1000).
		SetReferenceBlockID(latestBlock.ID).
		SetPayer(c.account.Address)

	// add signature data to the transaction and submit to node
	err = tx.AddArgument(cadence.NewString(hex.EncodeToString(vote.SigData)))
	if err != nil {
		return fmt.Errorf("could not add raw vote data to transaction: %w", err)
	}

	// sign payload using account signer
	err = tx.SignPayload(c.account.Address, c.accountKeyIndex, c.signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	// submit signed transaction to node
	txID, err := c.submitTx(tx)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	// wait for transaction to be sealed
	result := &sdk.TransactionResult{Status: sdk.TransactionStatusUnknown}
	for result.Status != sdk.TransactionStatusSealed {
		result, err = c.client.GetTransactionResult(ctx, txID)
		if err != nil {
			return fmt.Errorf("could not get transaction result: %w", err)
		}

		// if the transaction has expired we skip waiting for seal
		if result.Status == sdk.TransactionStatusExpired {
			break
		}

		// wait 1 second before trying again.
		time.Sleep(time.Second)
	}

	return nil
}

// Voted returns true if we have successfully submitted a vote to the
// cluster QC aggregator smart contract for the current epoch.
func (c *QCContractClient) Voted(ctx context.Context) (bool, error) {

	// template := templates.GenerateGetNodeHasVotedScript(c.getEnvironment())

	// val, err := jsoncdc.Decode(arg)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not deocde arguments: %w", err)
	// }

	// // execute script to read if voted
	// _, err := c.client.ExecuteScriptAtLatestBlock(ctx, template, []cadence.Value{val})
	// if err != nil {
	// 	return false, fmt.Errorf("could not execute voted script: %w", err)
	// }

	return false, nil
}

// submitTx submits a transaction to flow
func (c *QCContractClient) submitTx(tx *sdk.Transaction) (sdk.Identifier, error) {

	// check if the transaction has a signature
	if len(tx.EnvelopeSignatures) == 0 {
		return sdk.EmptyID, fmt.Errorf("can not submit an unsigned transaction")
	}

	// submit trnsaction to client
	err := c.client.SendTransaction(context.Background(), *tx)
	if err != nil {
		return sdk.EmptyID, fmt.Errorf("failed to send transaction: %w", err)
	}

	return tx.ID(), nil
}

func (c *QCContractClient) getEnvironment() templates.Environment {
	// environment to override transaction template contract addresses
	return templates.Environment{
		QuorumCertificateAddress: c.qcContractAddress,
	}
}
