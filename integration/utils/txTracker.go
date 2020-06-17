package main

import (
	"context"
	"fmt"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go-sdk/client"
)

// TODO add workers

type txInFlight struct {
	txID        flowsdk.Identifier
	lastStatus  flowsdk.TransactionStatus
	proposer    flowsdk.Address
	onError     func(flowsdk.Identifier, error)
	onSeal      func(flowsdk.Identifier, *flowsdk.TransactionResult)
	onFinalized func(flowsdk.Identifier, *flowsdk.TransactionResult)
	onTimeout   func(flowsdk.Identifier)
	createdAt   time.Time
	expiresAt   time.Time
}

type txTracker struct {
	client *client.Client
	txs    chan *txInFlight
}

// TODO pass port
func newTxTracker(maxCap int) (*txTracker, error) {
	fclient, err := client.New("localhost:3569", grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	txt := &txTracker{client: fclient,
		txs: make(chan *txInFlight, maxCap),
	}
	go txt.run()
	return txt, nil
}

func (txt *txTracker) addTx(txID flowsdk.Identifier,
	proposer flowsdk.Address,
	onErrorCallback func(flowsdk.Identifier, error),
	onSealCallback func(flowsdk.Identifier, *flowsdk.TransactionResult),
	onFinalizedCallback func(flowsdk.Identifier, *flowsdk.TransactionResult),
	onTimeoutCallback func(flowsdk.Identifier),
	timeoutInSec int,
) {
	result, _ := txt.client.GetTransactionResult(context.Background(), txID)
	// TODO deal with error
	newTx := &txInFlight{txID: txID,
		lastStatus:  result.Status,
		proposer:    proposer,
		onError:     onErrorCallback,
		onSeal:      onSealCallback,
		onFinalized: onFinalizedCallback,
		onTimeout:   onTimeoutCallback,
		createdAt:   time.Now(),
		expiresAt:   time.Now().Add(time.Duration(timeoutInSec)),
	}
	fmt.Println("tx added ", txID)
	txt.txs <- newTx
}

// TODO proper ready/done
func (txt *txTracker) stop() {
	close(txt.txs)
}

func (txt *txTracker) run() {
	for tx := range txt.txs {
		if tx.expiresAt.Before(time.Now()) {
			if tx.onTimeout != nil {
				go tx.onTimeout(tx.txID)
			}
			continue
		}
		fmt.Println("req sent for tx ", tx.txID)
		result, err := txt.client.GetTransactionResult(context.Background(), tx.txID)
		// TODO deal with error properly
		if err != nil {
			fmt.Println(err)
		}
		if result != nil {
			// if change in status
			if tx.lastStatus != result.Status {
				switch result.Status {
				case flowsdk.TransactionStatusFinalized:
					if tx.onFinalized != nil {
						go tx.onFinalized(tx.txID, result)
					}
					tx.lastStatus = flowsdk.TransactionStatusFinalized
					fmt.Println("tx ", tx.txID, "finalized in seconds: ", time.Since(tx.createdAt).Seconds)
				case flowsdk.TransactionStatusSealed:
					if tx.onSeal != nil {
						go tx.onSeal(tx.txID, result)
					}
					fmt.Println("tx ", tx.txID, "sealed in seconds: ", time.Since(tx.createdAt).Seconds)
					continue
				}
			}

		}
		// put it back
		txt.txs <- tx
		// TODO get rid of this
		time.Sleep(time.Second / 10)
	}
	fmt.Println("finished!")
}

// // TransactionStatusUnknown indicates that the transaction status is not known.
// TransactionStatusUnknown TransactionStatus = iota
// // TransactionStatusPending is the status of a pending transaction.
// TransactionStatusPending
// // TransactionStatusFinalized is the status of a finalized transaction.
// TransactionStatusFinalized
// // TransactionStatusExecuted is the status of an executed transaction.
// TransactionStatusExecuted
// // TransactionStatusSealed is the status of a sealed transaction.
// TransactionStatusSealed
// // TransactionStatusExpired is the status of an expired transaction.
// TransactionStatusExpired
