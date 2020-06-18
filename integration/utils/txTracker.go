package main

import (
	"context"
	"fmt"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go-sdk/client"
)

type txInFlight struct {
	txID        flowsdk.Identifier
	lastStatus  flowsdk.TransactionStatus
	onError     func(flowsdk.Identifier, error)
	onFinalized func(flowsdk.Identifier)
	onExecuted  func(flowsdk.Identifier, *flowsdk.TransactionResult)
	onSealed    func(flowsdk.Identifier, *flowsdk.TransactionResult)
	onExpired   func(flowsdk.Identifier)
	onTimeout   func(flowsdk.Identifier)
	createdAt   time.Time
	expiresAt   time.Time
}

type TxTracker struct {
	txs   chan *txInFlight
	stats *StatsTracker
}

func NewTxTracker(maxCap int,
	numberOfWorkers int,
	accessNodeAddress string,
	verbose bool,
	sleepAfterOp time.Duration,
	stats *StatsTracker) (*TxTracker, error) {

	txt := &TxTracker{txs: make(chan *txInFlight, maxCap), stats: stats}
	for i := 0; i < numberOfWorkers; i++ {
		fclient, err := client.New(accessNodeAddress, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		time.Sleep(sleepAfterOp) // creating parity
		go statusWorker(i, txt.txs, fclient, verbose, sleepAfterOp)
	}
	return txt, nil
}

func (txt *TxTracker) AddTx(txID flowsdk.Identifier,
	onFinalizedCallback func(flowsdk.Identifier),
	onExecutedCallback func(flowsdk.Identifier, *flowsdk.TransactionResult),
	onSealedCallback func(flowsdk.Identifier, *flowsdk.TransactionResult),
	onExpiredCallback func(flowsdk.Identifier),
	onTimeoutCallback func(flowsdk.Identifier),
	onErrorCallback func(flowsdk.Identifier, error),
	timeoutInSec int,
) {
	newTx := &txInFlight{txID: txID,
		lastStatus:  flowsdk.TransactionStatusUnknown,
		onError:     onErrorCallback,
		onFinalized: onFinalizedCallback,
		onExecuted:  onExecutedCallback,
		onSealed:    onSealedCallback,
		onExpired:   onExpiredCallback,
		onTimeout:   onTimeoutCallback,
		createdAt:   time.Now(),
		expiresAt:   time.Now().Add(time.Duration(timeoutInSec) * time.Second),
	}
	fmt.Println("tx added ", txID)
	txt.txs <- newTx
}

func (txt *TxTracker) stop() {
	// TODO use signals to shut down the workers or ready/done style
	close(txt.txs)
	time.Sleep(time.Second * 5)
}

// TODO enable verbose and do proper log formatting

func statusWorker(workerID int, txs chan *txInFlight, fclient *client.Client, verbose bool, sleepAfterOp time.Duration) {
	clientContErrorCounter := 0
	for tx := range txs {
		if clientContErrorCounter > 10 {
			fmt.Errorf("worker %d: calling client has failed for the last 10 times", workerID)
			break
		}
		if tx.expiresAt.Before(time.Now()) {
			if tx.onTimeout != nil {
				go tx.onTimeout(tx.txID)
			}
			if verbose {
				fmt.Printf("worker %d: status process for tx %v is timed out.\n", workerID, tx.txID)
			}
			continue
		}
		if verbose {
			fmt.Printf("worker %d: status update request sent for tx %v \n", workerID, tx.txID)
		}
		result, err := fclient.GetTransactionResult(context.Background(), tx.txID)
		if err != nil {
			clientContErrorCounter++
			txs <- tx
			continue
		}
		clientContErrorCounter = 0
		if result != nil {
			// if any change in status
			if tx.lastStatus != result.Status {
				switch result.Status {
				case flowsdk.TransactionStatusFinalized:
					if tx.onFinalized != nil {
						go tx.onFinalized(tx.txID)
					}
					tx.lastStatus = flowsdk.TransactionStatusFinalized
					if verbose {
						fmt.Printf("worker %d tx %v has been finalized in %v seconds\n", workerID, tx.txID, time.Since(tx.createdAt).Seconds())
					}
				case flowsdk.TransactionStatusExecuted:
					if tx.onExecuted != nil {
						go tx.onExecuted(tx.txID, result)
					}
					if verbose {
						fmt.Printf("worker %d tx %v has been executed in %v seconds\n", workerID, tx.txID, time.Since(tx.createdAt).Seconds())
					}
					tx.lastStatus = flowsdk.TransactionStatusExecuted

				case flowsdk.TransactionStatusSealed:
					if tx.onSealed != nil {
						go tx.onSealed(tx.txID, result)
					}
					if verbose {
						fmt.Printf("worker %d tx %v has been sealed in %v seconds\n", workerID, tx.txID, time.Since(tx.createdAt).Seconds())
					}
					continue

				case flowsdk.TransactionStatusUnknown:
					err := fmt.Errorf("worker %d tx %v got into an unknown status after %v seconds", workerID, tx.txID, time.Since(tx.createdAt).Seconds())
					if tx.onError != nil {
						go tx.onError(tx.txID, err)
					}
					if verbose {
						fmt.Println(err)
					}
					continue
				}
			}
		}
		// put it back
		txs <- tx
		// don't put too much pressure on access node
		time.Sleep(sleepAfterOp)
	}
	if verbose {
		fmt.Printf("worker %d stoped!\n", workerID)
	}

}
