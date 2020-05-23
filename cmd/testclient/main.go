package main

import (
	"context"
	"crypto/rand"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/pflag"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dapperlabs/flow-go-sdk/keys"
	"github.com/dapperlabs/flow-go/crypto"
)

var (
	targetAddr string
	txPerSec   int
)

func main() {
	pflag.StringVarP(&targetAddr, "target-address", "t", "localhost:9001", "address of the collection node to connect to")
	pflag.IntVarP(&txPerSec, "transaction-rate", "r", 1, "number of transactions to send per second")

	pflag.Parse()

	c, err := client.New(targetAddr)
	if err != nil {
		log.Fatal(err)
	}

	// Generate key
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)
	if n, err := rand.Read(seed); err != nil || n != crypto.KeyGenSeedMinLenECDSAP256 {
		log.Fatal(err)
	}
	key, err := keys.GeneratePrivateKey(keys.ECDSAP256_SHA2_256, seed)
	if err != nil {
		log.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	nonce := uint64(0)
	for {

		select {

		case <-time.After(time.Second / time.Duration(txPerSec)):

			nonce++

			tx := sdk.Transaction{
				Script:             []byte("fun main() {}"),
				ReferenceBlockHash: []byte{1, 2, 3, 4},
				Nonce:              nonce,
				ComputeLimit:       10,
				PayerAccount:       sdk.ServiceAddress(),
			}

			sig, err := keys.SignTransaction(tx, key)
			if err != nil {
				log.Fatal(err)
			}

			tx.AddSignature(sdk.ServiceAddress(), sig)

			err = c.SendTransaction(context.Background(), tx)
			if err != nil {
				log.Fatal(err)
			}

		case <-sig:
			os.Exit(0)
		}
	}
}
