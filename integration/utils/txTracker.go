package utils

import (
	"context"
	"fmt"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"google.golang.org/grpc"
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
	stat        *TxStats
}

// TxTracker activly calls access nodes and keep track of
// transactions by txID, in case of transaction state change
// it calls the provided callbacks
type TxTracker struct {
	numbOfWorkers int
	txs           chan *txInFlight
	done          chan bool
	stats         *StatsTracker
}

// NewTxTracker returns a new instance of TxTracker
func NewTxTracker(maxCap int,
	numberOfWorkers int,
	accessNodeAddress string,
	verbose bool,
	sleepAfterOp time.Duration,
	stats *StatsTracker) (*TxTracker, error) {

	txt := &TxTracker{
		numbOfWorkers: numberOfWorkers,
		txs:           make(chan *txInFlight, maxCap),
		done:          make(chan bool, numberOfWorkers),
		stats:         stats}
	for i := 0; i < numberOfWorkers; i++ {
		fclient, err := client.New(accessNodeAddress, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		time.Sleep(sleepAfterOp) // creating parity
		go statusWorker(i, txt.txs, txt.done, fclient, stats, verbose, sleepAfterOp)
	}
	return txt, nil
}

// AddTx adds a transaction to the tracker and register
// callbacks for transaction status changes
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
		stat:        &TxStats{isExpired: false},
	}
	fmt.Printf("%v tx added to tx tracker\n", txID)
	txt.txs <- newTx
}

// Stop stops the tracker workers
func (txt *TxTracker) Stop() {
	for i := 0; i < txt.numbOfWorkers; i++ {
		txt.done <- true
	}
	time.Sleep(time.Second * 5)
	close(txt.txs)
}

func statusWorker(workerID int, txs chan *txInFlight, done <-chan bool, fclient *client.Client, stats *StatsTracker, verbose bool, sleepAfterOp time.Duration) {
	clientContErrorCounter := 0

	for {
		select {
		case <-done:
			fmt.Printf("worker %d stoped!\n", workerID)
			return
		case tx := <-txs:

			if clientContErrorCounter > 10 {
				fmt.Printf("worker %d: calling client has failed for the last 10 times\n", workerID)
				return
			}
			if tx.expiresAt.Before(time.Now()) {
				if tx.onTimeout != nil {
					go tx.onTimeout(tx.txID)
				}
				if verbose {
					fmt.Printf("worker %d: status process for tx %v is timed out.\n", workerID, tx.txID)
				}
				tx.stat.isExpired = true
				stats.AddTxStats(tx.stat)
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
						tx.stat.TTF = time.Since(tx.createdAt)
						if verbose {
							fmt.Printf("worker %d tx %v has been finalized in %v seconds\n", workerID, tx.txID, time.Since(tx.createdAt).Seconds())
						}
					case flowsdk.TransactionStatusExecuted:
						if tx.onExecuted != nil {
							go tx.onExecuted(tx.txID, result)
						}
						tx.lastStatus = flowsdk.TransactionStatusExecuted
						tx.stat.TTE = time.Since(tx.createdAt)
						if verbose {
							fmt.Printf("worker %d tx %v has been executed in %v seconds\n", workerID, tx.txID, time.Since(tx.createdAt).Seconds())
						}
					case flowsdk.TransactionStatusSealed:
						if tx.onSealed != nil {
							go tx.onSealed(tx.txID, result)
						}

						tx.stat.TTS = time.Since(tx.createdAt)
						stats.AddTxStats(tx.stat)
						if verbose {
							fmt.Printf("worker %d tx %v has been sealed in %v seconds\n", workerID, tx.txID, time.Since(tx.createdAt).Seconds())
						}
						continue
					case flowsdk.TransactionStatusUnknown:
						err := fmt.Errorf("worker %d tx %v got into an unknown status after %v seconds", workerID, tx.txID, time.Since(tx.createdAt).Seconds())
						if tx.onError != nil {
							go tx.onError(tx.txID, err)
						}
						tx.stat.isExpired = true
						stats.AddTxStats(tx.stat)
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
	}
}
