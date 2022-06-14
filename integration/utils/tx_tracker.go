package utils

import (
	"context"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type txInFlight struct {
	txCallbacks

	txID       flowsdk.Identifier
	lastStatus flowsdk.TransactionStatus

	createdAt time.Time
	expiresAt time.Time
	stat      *TxStats

	wait chan struct{}
}

type txCallbacks struct {
	onExecuted  func(flowsdk.Identifier, *flowsdk.TransactionResult)
	onFinalized func(flowsdk.Identifier, *flowsdk.TransactionResult)
	onSealed    func(flowsdk.Identifier, *flowsdk.TransactionResult)
	onExpired   func(flowsdk.Identifier)
	onTimeout   func(flowsdk.Identifier)
}

// TxTracker activly calls access nodes and keep track of
// transactions by txID, in case of transaction state change
// it calls the provided callbacks
type TxTracker struct {
	cancel        context.CancelFunc
	log           zerolog.Logger
	numbOfWorkers int
	txs           chan *txInFlight
	clients       []*client.Client
	stats         *TxStatsTracker

	wg *sync.WaitGroup
}

// NewTxTracker returns a new instance of TxTracker
func NewTxTracker(
	log zerolog.Logger,
	maxCap int,
	numberOfWorkers int,
	accessNodeAddress string,
	sleepAfterOp time.Duration,
	stats *TxStatsTracker) (*TxTracker, error) {

	ctx, cancel := context.WithCancel(context.Background())
	txt := &TxTracker{
		cancel:        cancel,
		log:           log,
		numbOfWorkers: numberOfWorkers,
		clients:       make([]*client.Client, numberOfWorkers),
		txs:           make(chan *txInFlight, maxCap),
		stats:         stats,
		wg:            &sync.WaitGroup{},
	}

	for i := 0; i < numberOfWorkers; i++ {
		fclient, err := client.New(accessNodeAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			txt.Stop()
			return nil, err
		}
		txt.clients[i] = fclient

		txt.wg.Add(1)
		go txt.statusWorker(ctx, i, fclient, sleepAfterOp)
	}
	return txt, nil
}

// AddTx adds a transaction to the tracker and register
// callbacks for transaction status changes
// Returns a channel that will be closed when the transaction is removed from tracker
// (by either being sealed or failed (timeout, expiration, error))
func (txt *TxTracker) AddTx(
	txID flowsdk.Identifier,
	txCallbacks txCallbacks,
	timeout time.Duration,
) <-chan struct{} {
	newTx := &txInFlight{txID: txID,
		txCallbacks: txCallbacks,
		lastStatus:  flowsdk.TransactionStatusUnknown,
		createdAt:   time.Now(),
		expiresAt:   time.Now().Add(timeout),
		stat:        &TxStats{},
		wait:        make(chan struct{}),
	}
	txt.log.Trace().Str("tx_id", txID.String()).Msg("tx added to tx tracker")
	txt.txs <- newTx
	return newTx.wait
}

// Stop stops the tracker workers
func (txt *TxTracker) Stop() {
	txt.log.Info().Msg("stopping tx tracker")
	txt.cancel()

	for _, client := range txt.clients {
		if client != nil {
			_ = client.Close()
		}
	}

	txt.wg.Wait()

	close(txt.txs)
	txt.log.Info().Int("inflight", len(txt.txs)).Msg("cleaning up tracked transactions")
	for tx := range txt.txs {
		if tx.onTimeout != nil {
			go tx.onTimeout(tx.txID)
		}
		txt.log.Info().Msg("status process for tx was canceled")
		tx.stat.isExpired = true
		txt.stats.AddTxStats(tx.stat)
		close(tx.wait)
	}
}

func (txt *TxTracker) statusWorker(ctx context.Context, workerID int, fclient *client.Client, sleepAfterOp time.Duration) {
	log := txt.log.With().Int("worker_id", workerID).Logger()
	log.Trace().Msg("worker started")

	defer txt.wg.Done()

	for {
		select {
		case <-ctx.Done():
			log.Debug().Msg("worker stopped")
			return
		case tx := <-txt.txs:
			log = log.With().Str("tx_id", tx.txID.String()).Logger()
			log.Trace().Msg("took tx from queue")

			if tx.expiresAt.Before(time.Now()) {
				if tx.onTimeout != nil {
					go tx.onTimeout(tx.txID)
				}
				log.Info().Msg("status process for tx timed out")
				tx.stat.isExpired = true
				txt.stats.AddTxStats(tx.stat)
				close(tx.wait)
				continue
			}
			log.Trace().Msg("status update request sent for tx")

			result, err := fclient.GetTransactionResult(ctx, tx.txID)
			if err != nil {
				log.Warn().Err(err).Msg("error getting tx result, will put tx back to queue after sleep")

				// don't put too much pressure on access node
				time.Sleep(sleepAfterOp)

				txt.txs <- tx
				continue
			}
			// if any change in status
			if tx.lastStatus != result.Status {
				log = log.With().Dur("duration", time.Since(tx.createdAt)).Logger()
				switch result.Status {
				case flowsdk.TransactionStatusFinalized:
					log.Trace().Msg("tx has been finalized")
					if tx.onFinalized != nil {
						go tx.onFinalized(tx.txID, result)
					}
					tx.lastStatus = flowsdk.TransactionStatusFinalized
					tx.stat.TTF = time.Since(tx.createdAt)
				case flowsdk.TransactionStatusExecuted:
					log.Trace().Msg("tx has been executed")
					if tx.onExecuted != nil {
						go tx.onExecuted(tx.txID, result)
					}
					tx.lastStatus = flowsdk.TransactionStatusExecuted
					tx.stat.TTE = time.Since(tx.createdAt)
				case flowsdk.TransactionStatusUnknown:
					log.Trace().Msg("got into an unknown status, retrying")
				case flowsdk.TransactionStatusSealed:
					log.Trace().Msg("tx has been sealed")
					// sometimes we miss the executed call and just get sealed directly
					if tx.lastStatus != flowsdk.TransactionStatusExecuted {
						if tx.onExecuted != nil {
							go tx.onExecuted(tx.txID, result)
						}
					}
					if tx.onSealed != nil {
						go tx.onSealed(tx.txID, result)
					}

					tx.stat.TTS = time.Since(tx.createdAt)
					txt.stats.AddTxStats(tx.stat)
					close(tx.wait)
					continue
				case flowsdk.TransactionStatusExpired:
					log.Warn().Msg("tx has been expired")
					if tx.onExpired != nil {
						go tx.onExpired(tx.txID)
					}
					tx.stat.isExpired = true
					txt.stats.AddTxStats(tx.stat)
					close(tx.wait)
					continue
				}
			}

			log.Trace().Msg("putting tx back to queue")

			// don't put too much pressure on access node
			time.Sleep(sleepAfterOp)

			// put it back
			txt.txs <- tx
		}
	}
}
