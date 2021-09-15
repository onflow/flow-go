package utils

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/rlp"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"time"
)

const (
	LocalnetServiceAccountPrivateKeyHex = "e3a08ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc430201"
)

// accountPrivateKeyWrapper is used for encoding and decoding.
type accountPrivateKeyWrapper struct {
	PrivateKey []byte
	SignAlgo   uint
	HashAlgo   uint
}

type ServiceAccountDetails struct {
	Signer                crypto.Signer
	ServiceAccountAddress sdk.Address
	ServiceAccountKeyID   int
	PrivateKey            crypto.PrivateKey
}

func LocalnetService() (*ServiceAccountDetails, error) {
	servicePrivateKey, err := AccountPrivateKeyFromHex(LocalnetServiceAccountPrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("could not read service private key %w", err)
	}

	_, serviceSigner := FromPrivateKey(crypto.SHA2_256, servicePrivateKey)
	serviceAccountKeyID := 0
	serviceAccountAddress := sdk.HexToAddress("0xf8d6e0586b0a20c7")

	return &ServiceAccountDetails{
		Signer: serviceSigner,
		ServiceAccountAddress: serviceAccountAddress,
		ServiceAccountKeyID: serviceAccountKeyID,
		PrivateKey: servicePrivateKey,
	}, nil
}

func AccountPrivateKeyFromHex(keyHex string) (crypto.PrivateKey, error) {

	skBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("could not decode private key from hex: %v", err)
	}

	var w accountPrivateKeyWrapper
	err = rlp.DecodeBytes(skBytes, &w)
	if err != nil {
		return nil, fmt.Errorf("could not rlp decode bytes: %w", err)
	}

	signingAlgo := crypto.ECDSA_P256

	sk, err := crypto.DecodePrivateKey(signingAlgo, w.PrivateKey)

	return sk, err
}

func FromPrivateKey(hashAlgo crypto.HashAlgorithm, sk crypto.PrivateKey) (*sdk.AccountKey, crypto.Signer) {
	accountKey := AccountKeyFromPrivateKey(sk)
	signer := crypto.NewInMemorySigner(sk, hashAlgo)
	return accountKey, signer
}

func AccountKeyFromPrivateKey(sk crypto.PrivateKey) *sdk.AccountKey {
	return AccountKeyFromPublicKey(sk.PublicKey())
}

func AccountKeyFromPublicKey(pk crypto.PublicKey) *sdk.AccountKey {
	hashAlgo := crypto.SHA2_256
	return sdk.NewAccountKey().
		SetPublicKey(pk).
		SetHashAlgo(hashAlgo).
		SetWeight(sdk.AccountKeyWeightThreshold)
}

func GetAccount(flowClient *client.Client, accountAddress sdk.Address) (*sdk.Account, error) {
	ctx := context.Background()
	account, err := flowClient.GetAccount(ctx, accountAddress)
	if err != nil {
		return nil, fmt.Errorf("could not get service account: %w", err)
	}

	return account, nil
}

func SignTx(tx sdk.Transaction, signer crypto.Signer, signerKeyID int, signerAddress sdk.Address) (sdk.Transaction, []byte, error) {
	err := tx.SignEnvelope(signerAddress, signerKeyID, signer)
	if err != nil {
		return sdk.Transaction{}, nil, fmt.Errorf("failed to sign transaction envelope: %w", err)
	}

	sig, found := getSignature(&tx)
	if !found {
		return sdk.Transaction{}, nil, fmt.Errorf("signature not found after signed")
	}

	return tx, sig, nil
}

func getSignature(tx *sdk.Transaction) ([]byte, bool) {
	if len(tx.EnvelopeSignatures) == 0 {
		return nil, false
	}
	return tx.EnvelopeSignatures[0].Signature, true
}

func SignSubmitAndWaitForSealed(unsignedTx sdk.Transaction, signerAccount *sdk.Account, signerKeyID int, signer crypto.Signer, client *client.Client) (*sdk.TransactionResult, error) {
	signedTx, _, err := SignTx(unsignedTx, signer, signerKeyID, signerAccount.Address)
	if err != nil {
		return nil, fmt.Errorf("could not sign transaction %w", err)
	}

	signerKey := signerAccount.Keys[signerKeyID]
	signerKey.SequenceNumber++

	argSize := 0
	for _, arg := range signedTx.Arguments {
		argSize += len(arg)
	}

	addrSize := 0
	for _, arg := range signedTx.Authorizers {
		addrSize += len(arg)
	}

	txHash, err := SubmitSignedTx(client, signedTx)
	if err != nil {
		return nil, fmt.Errorf("could not submit signed transaction %w", err)
	}

	txResult, err := WaitForSeal(client, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to wait tx sealed %w", err)
	}

	return txResult, nil
}

func SubmitSignedTx(flowClient *client.Client, tx sdk.Transaction) (sdk.Identifier, error) {
	_, found := getSignature(&tx)
	if !found {
		return sdk.EmptyID, fmt.Errorf("can't submit a unsigned transaction")
	}

	ctx := context.Background()
	err := flowClient.SendTransaction(ctx, tx)
	if err != nil {
		return sdk.EmptyID, fmt.Errorf("failed to send transaction: %w", err)
	}
	txHash := tx.ID()
	return txHash, nil
}

func WaitForSeal(c *client.Client, txHash sdk.Identifier) (*sdk.TransactionResult, error) {
	result := &sdk.TransactionResult{Status: sdk.TransactionStatusUnknown}
	var err error
	for result.Status != sdk.TransactionStatusSealed {
		ctx := context.Background()
		result, err = c.GetTransactionResult(ctx, txHash)
		if err != nil {
			return nil, fmt.Errorf("could not get transaction result:%w", err)
		}
		time.Sleep(time.Second)
	}

	return result, nil
}

func UserAddress(txResp *sdk.TransactionResult) (sdk.Address, bool) {
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
