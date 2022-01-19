package utils

import (
	"context"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/rs/zerolog"
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
	log           zerolog.Logger
	numbOfWorkers int
	txs           chan *txInFlight
	done          chan bool
	stats         *TxStatsTracker
}

// NewTxTracker returns a new instance of TxTracker
func NewTxTracker(
	log zerolog.Logger,
	maxCap int,
	numberOfWorkers int,
	accessNodeAddress string,
	sleepAfterOp time.Duration,
	stats *TxStatsTracker) (*TxTracker, error) {

	txt := &TxTracker{
		log:           log,
		numbOfWorkers: numberOfWorkers,
		txs:           make(chan *txInFlight, maxCap),
		done:          make(chan bool, numberOfWorkers),
		stats:         stats}
	for i := 0; i < numberOfWorkers; i++ {
		fclient, err := client.New(accessNodeAddress, grpc.WithInsecure()) //nolint:staticcheck
		if err != nil {
			return nil, err
		}
		go statusWorker(log, i, txt.txs, txt.done, fclient, stats, sleepAfterOp)
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
	txt.log.Debug().Str("tx_id", txID.String()).Msg("tx added to tx tracker")
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

func statusWorker(log zerolog.Logger, workerID int, txs chan *txInFlight, done <-chan bool, fclient *client.Client, stats *TxStatsTracker, sleepAfterOp time.Duration) {
	log.Debug().Int("worker_id", workerID).Msg("worker started")

	clientContErrorCounter := 0

	for {
		select {
		case <-done:
			log.Debug().Int("worker_id", workerID).Msg("worker stopped")
			return
		case tx := <-txs:
			log.Trace().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).Msg("took tx from queue")

			if clientContErrorCounter > 10 {
				log.Error().Int("worker_id", workerID).Msg("calling client has failed for the last 10 times")
				return
			}
			if tx.expiresAt.Before(time.Now()) {
				if tx.onTimeout != nil {
					go tx.onTimeout(tx.txID)
				}
				log.Info().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).
					Msg("status process for tx timed out")
				tx.stat.isExpired = true
				stats.AddTxStats(tx.stat)
				continue
			}
			log.Trace().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).
				Msg("status update request sent for tx")
			result, err := fclient.GetTransactionResult(context.Background(), tx.txID)
			if err != nil {
				clientContErrorCounter++
				log.Warn().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).Err(err).
					Msg("error getting tx result, will put tx back to queue after sleep")

				// don't put too much pressure on access node
				time.Sleep(sleepAfterOp)

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
						log.Trace().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).
							Msgf("tx has been finalized in %v seconds", time.Since(tx.createdAt).Seconds())
					case flowsdk.TransactionStatusExecuted:
						if tx.onExecuted != nil {
							go tx.onExecuted(tx.txID, result)
						}
						tx.lastStatus = flowsdk.TransactionStatusExecuted
						tx.stat.TTE = time.Since(tx.createdAt)
						log.Trace().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).
							Msgf("tx has been executed in %v seconds", time.Since(tx.createdAt).Seconds())
					case flowsdk.TransactionStatusSealed:
						// sometimes we miss the executed call and just get sealed directly
						if tx.lastStatus != flowsdk.TransactionStatusExecuted {
							if tx.onExecuted != nil {
								go tx.onExecuted(tx.txID, result)
							}
							// TODO fix me
							// tx.stat.TTE = time.Since(tx.createdAt)
						}
						if tx.onSealed != nil {
							go tx.onSealed(tx.txID, result)
						}

						tx.stat.TTS = time.Since(tx.createdAt)
						stats.AddTxStats(tx.stat)
						log.Trace().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).
							Msgf("tx has been sealed in %v seconds", time.Since(tx.createdAt).Seconds())
						continue
					case flowsdk.TransactionStatusUnknown:
						log.Warn().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).
							Msgf("got into an unknown status after %v seconds", time.Since(tx.createdAt).Seconds())
						if tx.onError != nil {
							go tx.onError(tx.txID, err)
						}
						tx.stat.isExpired = true
						stats.AddTxStats(tx.stat)
						continue
					case flowsdk.TransactionStatusExpired:
						if tx.onExpired != nil {
							go tx.onExpired(tx.txID)
						}
						tx.stat.isExpired = true
						stats.AddTxStats(tx.stat)
						log.Warn().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).
							Msgf("tx has been expired in %v seconds", time.Since(tx.createdAt).Seconds())
						continue
					}
				}
			}

			log.Trace().Int("worker_id", workerID).Str("tx_id", tx.txID.String()).
				Msg("empty result, putting tx back to queue")

			// don't put too much pressure on access node
			time.Sleep(sleepAfterOp)

			// put it back
			txs <- tx
		}
	}
}
