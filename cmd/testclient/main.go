package main

import (
	"context"
	"crypto/rand"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
)

var (
	targetAddr string
	txPerSec   int
)

func main() {
	pflag.StringVarP(&targetAddr, "target-address", "t", "localhost:9001", "address of the collection node to connect to")
	pflag.IntVarP(&txPerSec, "transaction-rate", "r", 1, "number of transactions to send per second")

	pflag.Parse()

	c, err := client.New(targetAddr, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	// Generate key
	seed := make([]byte, crypto.MinSeedLength)
	_, err = rand.Read(seed)
	if err != nil {
		panic(err)
	}

	sk, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, seed)
	if err != nil {
		panic(err)
	}

	account := sdk.NewAccountKey().
		FromPrivateKey(sk).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	signer := crypto.NewInMemorySigner(sk, account.HashAlgo)

	addr := sdk.NewAddressGenerator(sdk.Testnet).NextAddress()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	nonce := uint64(0)
	for {

		select {

		case <-time.After(time.Second / time.Duration(txPerSec)):

			nonce++

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			latest, err := c.GetLatestBlockHeader(ctx, false)
			if err != nil {
				panic(err)
			}

			tx := sdk.NewTransaction().
				SetScript([]byte(`
            		transaction { 
            		    prepare(signer: AuthAccount) { log(signer.address) }
            		}
        		`)).
				SetGasLimit(100).
				SetProposalKey(addr, account.ID, nonce).
				SetReferenceBlockID(latest.ID).
				SetPayer(addr).
				AddAuthorizer(addr)

			err = tx.SignEnvelope(addr, 1, signer)
			if err != nil {
				panic(err)
			}

			err = c.SendTransaction(ctx, *tx)
			if err != nil {
				panic(err)
			}

			cancel()

		case <-sig:
			os.Exit(0)
		}
	}
}
