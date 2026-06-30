package testnet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/onflow/cadence"
	"github.com/onflow/crypto/hash"
	sdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/dsl"
	"github.com/onflow/flow-go/utils/unittest"
)

// Client is a GRPC client of the Access API exposed by the Flow network.
// NOTE: we use integration/client rather than sdk/client as a stopgap until
// the SDK client is updated with the latest protobuf definitions.
type Client struct {
	client         *client.Client
	accountKey     *sdk.AccountKey
	accountKeyPriv sdkcrypto.PrivateKey
	signer         sdkcrypto.InMemorySigner
	Chain          flow.Chain
	account        *sdk.Account
}

// NewClientWithKey returns a new client to an Access API listening at the given
// address, using the given account key for signing transactions.
func NewClientWithKey(accessAddr string, accountAddr sdk.Address, key sdkcrypto.PrivateKey, chain flow.Chain) (*Client, error) {
	flowClient, err := client.NewClient(
		accessAddr,
		client.WithGRPCDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create new flow client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	acc, err := getAccount(ctx, flowClient, accountAddr)
	if err != nil {
		return nil, fmt.Errorf("could not get the account %v: %w", accountAddr, err)
	}

	accountKey := acc.Keys[0]

	mySigner, err := sdkcrypto.NewInMemorySigner(key, accountKey.HashAlgo)
	if err != nil {
		return nil, fmt.Errorf("could not create a signer: %w", err)
	}

	tc := &Client{
		client:         flowClient,
		accountKey:     accountKey,
		accountKeyPriv: key,
		signer:         mySigner,
		Chain:          chain,
		account:        acc,
	}
	return tc, nil
}

// NewClient returns a new client to an Access API listening at the given
// address, with a test service account key for signing transactions.
func NewClient(addr string, chain flow.Chain) (*Client, error) {
	key := unittest.ServiceAccountPrivateKey
	privateKey, err := sdkcrypto.DecodePrivateKey(sdkcrypto.SignatureAlgorithm(key.SignAlgo), key.PrivateKey.Encode())
	if err != nil {
		return nil, fmt.Errorf("could not decode private key: %w", err)
	}
	// Uncomment for debugging keys

	// json, err := key.MarshalJSON()
	// if err != nil {
	//	return nil, fmt.Errorf("cannot marshal key json: %w", err)
	// }
	// public := key.PublicKey(1000)
	// publicJson, err := public.MarshalJSON()
	// if err != nil {
	//	return nil, fmt.Errorf("cannot marshal key json: %w", err)
	// }
	//
	// fmt.Printf("New client with private key: \n%s\n", json)
	// fmt.Printf("and public key: \n%s\n", publicJson)

	return NewClientWithKey(addr, sdk.Address(chain.ServiceAddress()), privateKey, chain)
}

// AccountKeyPriv returns the private used to create the in memory signer for the account
func (c *Client) AccountKeyPriv() sdkcrypto.PrivateKey {
	return c.accountKeyPriv
}

func (c *Client) GetAndIncrementSeqNumber() uint64 {
	n := c.accountKey.SequenceNumber
	c.accountKey.SequenceNumber++
	return n
}

func (c *Client) Events(ctx context.Context, typ string) ([]sdk.BlockEvents, error) {
	return c.client.GetEventsForHeightRange(ctx, typ, 0, 1000)
}

// DeployContract submits a transaction to deploy a contract with the given
// code to the root account.
func (c *Client) DeployContract(ctx context.Context, refID sdk.Identifier, contract dsl.Contract) (*sdk.Transaction, error) {
	return c.deployContract(ctx, refID, dsl.Transaction{
		Content: dsl.Prepare{
			Content: dsl.SetAccountCode{
				Code: contract.ToCadence(),
				Name: contract.Name,
			},
		},
	})
}

// UpdateContract submits a transaction to deploy a contract update with the given
// code to the root account.
func (c *Client) UpdateContract(ctx context.Context, refID sdk.Identifier, contract dsl.Contract) (*sdk.Transaction, error) {
	return c.deployContract(ctx, refID, dsl.Transaction{
		Content: dsl.Prepare{
			Content: dsl.SetAccountCode{
				Code:   contract.ToCadence(),
				Name:   contract.Name,
				Update: true,
			},
		},
	})
}

func (c *Client) deployContract(ctx context.Context, refID sdk.Identifier, code dsl.Transaction) (*sdk.Transaction, error) {
	tx := sdk.NewTransaction().
		SetScript([]byte(code.ToCadence())).
		SetReferenceBlockID(refID).
		SetProposalKey(c.SDKServiceAddress(), 0, c.GetAndIncrementSeqNumber()).
		SetPayer(c.SDKServiceAddress()).
		AddAuthorizer(c.SDKServiceAddress())

	err := c.SignAndSendTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("could not deploy contract: %w", err)
	}

	return tx, nil
}

// SignTransaction signs the transaction using the proposer's key
func (c *Client) SignTransaction(tx *sdk.Transaction) (*sdk.Transaction, error) {

	err := tx.SignEnvelope(tx.Payer, tx.ProposalKey.KeyIndex, c.signer)
	if err != nil {
		return nil, err
	}

	return tx, err
}

func (c *Client) SignTransactionWebAuthN(tx *sdk.Transaction) (*sdk.Transaction, error) {
	transactionMessage := tx.EnvelopeMessage()

	extensionData, messageToSign, err := c.validWebAuthnExtensionData(transactionMessage)
	if err != nil {
		return nil, err
	}
	sig, err := c.signer.Sign(messageToSign)
	if err != nil {
		return nil, err
	}
	tx.AddEnvelopeSignature(tx.Payer, tx.ProposalKey.KeyIndex, sig)
	tx.EnvelopeSignatures[0].ExtensionData = slices.Concat([]byte{byte(flow.WebAuthnScheme)}, extensionData)
	return tx, nil
}

func (c *Client) validWebAuthnExtensionData(transactionMessage []byte) ([]byte, []byte, error) {
	hasher, err := crypto.NewPrefixedHashing(hash.SHA2_256, flow.TransactionTagString)
	if err != nil {
		return nil, nil, err
	}
	authNChallenge := hasher.ComputeHash(transactionMessage)
	authNChallengeBase64Url := base64.RawURLEncoding.EncodeToString(authNChallenge)
	validUserFlag := byte(0x01)
	validClientDataOrigin := "https://testing.com"
	rpIDHash := unittest.RandomBytes(32)
	sigCounter := unittest.RandomBytes(4)

	// For use in cases where you're testing the other value
	validAuthenticatorData := slices.Concat(rpIDHash, []byte{validUserFlag}, sigCounter)
	validClientDataJSON := map[string]string{
		"type":      flow.WebAuthnTypeGet,
		"challenge": authNChallengeBase64Url,
		"origin":    validClientDataOrigin,
	}

	clientDataJsonBytes, err := json.Marshal(validClientDataJSON)
	if err != nil {
		return nil, nil, err
	}

	extensionData := flow.WebAuthnExtensionData{
		AuthenticatorData: validAuthenticatorData,
		ClientDataJson:    clientDataJsonBytes,
	}
	extensionDataRLPBytes := rlp.NewMarshaler().MustMarshal(extensionData)

	var clientDataHash [hash.HashLenSHA2_256]byte
	hash.ComputeSHA2_256(&clientDataHash, clientDataJsonBytes)
	messageToSign := slices.Concat(validAuthenticatorData, clientDataHash[:])

	return extensionDataRLPBytes, messageToSign, nil
}

// SendTransaction submits the transaction to the Access API. The caller must
// set up the transaction, including signing it.
func (c *Client) SendTransaction(ctx context.Context, tx *sdk.Transaction) error {
	return c.client.SendTransaction(ctx, *tx)
}

// SignAndSendTransaction signs, then sends, a transaction. I bet you didn't
// see that one coming.
func (c *Client) SignAndSendTransaction(ctx context.Context, tx *sdk.Transaction) error {
	tx, err := c.SignTransaction(tx)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	return c.SendTransaction(ctx, tx)
}

func (c *Client) GetTransactionResult(ctx context.Context, txID sdk.Identifier) (*sdk.TransactionResult, error) {
	return c.client.GetTransactionResult(ctx, txID)
}

func (c *Client) ExecuteScript(ctx context.Context, script dsl.Main) (cadence.Value, error) {

	code := script.ToCadence()

	res, err := c.client.ExecuteScriptAtLatestBlock(ctx, []byte(code), nil)
	if err != nil {
		return nil, fmt.Errorf("could not execute script: %w", err)
	}

	return res, nil
}

func (c *Client) ExecuteScriptAtBlock(ctx context.Context, script dsl.Main, blockID sdk.Identifier) (cadence.Value, error) {
	res, err := c.client.ExecuteScriptAtBlockID(ctx, blockID, []byte(script.ToCadence()), nil)
	if err != nil {
		return nil, fmt.Errorf("could not execute script: %w", err)
	}

	return res, nil
}

func (c *Client) ExecuteScriptBytes(ctx context.Context, script []byte, args []cadence.Value) (cadence.Value, error) {
	res, err := c.client.ExecuteScriptAtLatestBlock(ctx, script, args)
	if err != nil {
		return nil, fmt.Errorf("could not execute script: %w", err)
	}

	return res, nil
}

func (c *Client) SDKServiceAddress() sdk.Address {
	return sdk.Address(c.Chain.ServiceAddress())
}

// AccountKey returns the flow account key for the client
func (c *Client) AccountKey() *sdk.AccountKey {
	return c.accountKey
}

func (c *Client) Account() *sdk.Account {
	return c.account
}

// WaitForFinalized waits for the transaction to be finalized, then returns the result.
func (c *Client) WaitForFinalized(ctx context.Context, id sdk.Identifier) (*sdk.TransactionResult, error) {
	return c.waitForStatus(ctx, id, sdk.TransactionStatusFinalized)
}

// WaitForSealed waits for the transaction to be sealed, then returns the result.
func (c *Client) WaitForSealed(ctx context.Context, id sdk.Identifier) (*sdk.TransactionResult, error) {
	return c.waitForStatus(ctx, id, sdk.TransactionStatusSealed)
}

// WaitForExecuted waits for the transaction to be executed, then returns the result.
func (c *Client) WaitForExecuted(ctx context.Context, id sdk.Identifier) (*sdk.TransactionResult, error) {
	return c.waitForStatus(ctx, id, sdk.TransactionStatusExecuted)
}

// WaitUntilIndexed blocks until the node has indexed the given height.
func (c *Client) WaitUntilIndexed(ctx context.Context, height uint64) error {
	for {
		resp, err := c.client.RPCClient().GetLatestBlockHeader(ctx, &accessproto.GetLatestBlockHeaderRequest{
			IsSealed: true,
		})
		if err != nil {
			return fmt.Errorf("could not get metadata response: %w", err)
		}
		if resp.GetMetadata().GetHighestIndexedHeight() >= height {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// waitForStatus waits for the transaction to be in a certain status, then returns the result.
func (c *Client) waitForStatus(
	ctx context.Context,
	id sdk.Identifier,
	targetStatus sdk.TransactionStatus,
) (*sdk.TransactionResult, error) {
	fmt.Printf("Waiting for transaction %s to be %v...\n", id, targetStatus)
	errCount := 0
	var result *sdk.TransactionResult
	var err error
	for result == nil || (result.Status != targetStatus) {
		childCtx, cancel := context.WithTimeout(ctx, time.Second*30)
		result, err = c.client.GetTransactionResult(childCtx, id)
		cancel()
		if err != nil {
			fmt.Print("x")
			errCount++
			if errCount >= 10 {
				return &sdk.TransactionResult{
					Error: err,
				}, err
			}
		} else {
			fmt.Print(".")
		}
		time.Sleep(250 * time.Millisecond)
	}

	fmt.Println()
	fmt.Printf("(Wait for Seal) Transaction %s %s\n", id, targetStatus)

	return result, err
}

// Ping sends a ping request to the node
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx)
}

// GetLatestProtocolSnapshot returns the latest protocol state snapshot.
// The snapshot head is latest finalized - tail of sealing segment is latest sealed.
func (c *Client) GetLatestProtocolSnapshot(ctx context.Context) (*inmem.Snapshot, error) {
	b, err := c.client.GetLatestProtocolStateSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get latest snapshot from access node: %v", err)
	}

	snapshot, err := convert.BytesToInmemSnapshot(b)
	if err != nil {
		return nil, fmt.Errorf("could not convert bytes to snapshot: %v", err)
	}

	return snapshot, nil
}

// GetLatestBlockID returns block ID of the latest sealed block
func (c *Client) GetLatestBlockID(ctx context.Context) (flow.Identifier, error) {
	header, err := c.client.GetLatestBlockHeader(ctx, true)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get latest sealed block header: %w", err)
	}

	var id flow.Identifier
	copy(id[:], header.ID[:])
	return id, nil
}

// GetLatestSealedBlockHeader returns full block header for the latest sealed block
func (c *Client) GetLatestSealedBlockHeader(ctx context.Context) (*sdk.BlockHeader, error) {
	header, err := c.client.GetLatestBlockHeader(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("could not get latest sealed block header: %w", err)
	}

	return header, nil
}

// GetLatestFinalizedBlockHeader returns full block header for the latest finalized block
func (c *Client) GetLatestFinalizedBlockHeader(ctx context.Context) (*sdk.BlockHeader, error) {
	header, err := c.client.GetLatestBlockHeader(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("could not get latest sealed block header: %w", err)
	}

	return header, nil
}

func (c *Client) UserAddress(txResp *sdk.TransactionResult) (sdk.Address, bool) {
	var (
		address sdk.Address
		found   bool
	)

	// For account creation transactions that create multiple accounts, assume the last
	// created account is the user account. This is specifically the case for the
	// locked token shared account creation transactions.
	for _, event := range txResp.Events {
		if event.Type == sdk.EventAccountCreated {
			accountCreatedEvent := sdk.AccountCreatedEvent(event)
			address = accountCreatedEvent.Address()
			found = true
		}
	}

	return address, found
}

// CreatedAccounts returns the addresses of all accounts created in the given transaction,
// in the order they were created.
func (c *Client) CreatedAccounts(txResp *sdk.TransactionResult) []sdk.Address {
	var addresses []sdk.Address
	for _, event := range txResp.Events {
		if event.Type == sdk.EventAccountCreated {
			accountCreatedEvent := sdk.AccountCreatedEvent(event)
			addresses = append(addresses, accountCreatedEvent.Address())
		}
	}
	return addresses
}

func (c *Client) TokenAmountByRole(role flow.Role) (string, float64, error) {
	if role == flow.RoleCollection {
		return "250000.0", 250000.0, nil
	}
	if role == flow.RoleConsensus {
		return "500000.0", 500000.0, nil
	}
	if role == flow.RoleExecution {
		return "1250000.0", 1250000.0, nil
	}
	if role == flow.RoleVerification {
		return "135000.0", 135000.0, nil
	}
	if role == flow.RoleAccess {
		return "100.0", 100.0, nil
	}

	return "", 0, fmt.Errorf("could not get token amount by role: %v", role)
}

func (c *Client) GetAccount(accountAddress sdk.Address) (*sdk.Account, error) {
	ctx := context.Background()
	account, err := c.client.GetAccount(ctx, accountAddress)
	if err != nil {
		return nil, fmt.Errorf("could not get account: %w", err)
	}

	return account, nil
}

func (c *Client) GetAccountAtBlockHeight(ctx context.Context, accountAddress sdk.Address, blockHeight uint64) (*sdk.Account, error) {
	account, err := c.client.GetAccountAtBlockHeight(ctx, accountAddress, blockHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get account at block height: %w", err)
	}
	return account, nil
}

func (c *Client) CreateAccount(
	ctx context.Context,
	accountKey *sdk.AccountKey,
	latestBlockID sdk.Identifier,
) (sdk.Address, *sdk.TransactionResult, error) {
	payer := c.SDKServiceAddress()
	tx, err := templates.CreateAccount([]*sdk.AccountKey{accountKey}, nil, payer)
	if err != nil {
		return sdk.Address{}, nil, fmt.Errorf("failed to construct create account transaction: %w", err)
	}
	tx.SetComputeLimit(1000).
		SetReferenceBlockID(latestBlockID).
		SetProposalKey(payer, 0, c.GetAndIncrementSeqNumber()).
		SetPayer(payer)

	err = c.SignAndSendTransaction(ctx, tx)
	if err != nil {
		return sdk.Address{}, nil, fmt.Errorf("failed to sign and send create account transaction %w", err)
	}

	result, err := c.WaitForSealed(ctx, tx.ID())
	if err != nil {
		return sdk.Address{}, nil, fmt.Errorf("failed to wait for create account transaction to seal %w", err)
	}

	if result.Error != nil {
		return sdk.Address{}, nil, fmt.Errorf("failed to create new account %w", result.Error)
	}

	if address, ok := c.UserAddress(result); ok {
		return address, result, nil
	}

	return sdk.Address{}, nil, fmt.Errorf("failed to get account address of the created flow account")
}

func (c *Client) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []sdk.Identifier,
) ([]sdk.BlockEvents, error) {
	events, err := c.client.GetEventsForBlockIDs(ctx, eventType, blockIDs)
	if err != nil {
		return nil, fmt.Errorf("could not get events for block ids: %w", err)
	}

	return events, nil
}

func (c *Client) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight uint64,
	endHeight uint64,
) ([]sdk.BlockEvents, error) {
	events, err := c.client.GetEventsForHeightRange(ctx, eventType, startHeight, endHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get events for height range: %w", err)
	}
	return events, nil
}

func getAccount(ctx context.Context, client *client.Client, address sdk.Address) (*sdk.Account, error) {
	header, err := client.GetLatestBlockHeader(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("could not get latest block header: %w", err)
	}

	// when this is run against an Access node with indexing enabled, occasionally the indexed height
	// lags behind the sealed height, especially after first starting up (like in a test).
	// Retry using the same block until we get the account.
	for {
		acc, err := client.GetAccountAtBlockHeight(ctx, address, header.Height)
		if err == nil {
			return acc, nil
		}

		switch status.Code(err) {
		case codes.OutOfRange, codes.ResourceExhausted:
			time.Sleep(100 * time.Millisecond)
			continue
		}
		return nil, fmt.Errorf("could not get the account %v: %w", address, err)
	}
}
