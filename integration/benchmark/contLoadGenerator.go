package benchmark

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/benchmark/account"
	"github.com/onflow/flow-go/integration/benchmark/common"
	"github.com/onflow/flow-go/integration/benchmark/load"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

const lostTransactionThreshold = 180 * time.Second

// ContLoadGenerator creates a continuous load of transactions to the network
// by creating many accounts and transfer flow tokens between them
type ContLoadGenerator struct {
	ctx context.Context

	log                zerolog.Logger
	loaderMetrics      *metrics.LoaderCollector
	loadParams         LoadParams
	flowClient         access.Client
	workerStatsTracker *WorkerStatsTracker
	stoppedChannel     chan struct{}
	follower           TxFollower
	workFunc           workFunc

	workersMutex sync.Mutex
	workers      []*Worker
}

type NetworkParams struct {
	ServAccPrivKeyHex string
	ChainId           flow.ChainID
}

type LoadParams struct {
	NumberOfAccounts int
	LoadType         load.LoadType

	// TODO(rbtz): inject a TxFollower
	FeedbackEnabled bool
}

// New returns a new load generator
func New(
	ctx context.Context,
	log zerolog.Logger,
	workerStatsTracker *WorkerStatsTracker,
	loaderMetrics *metrics.LoaderCollector,
	flowClients []access.Client,
	networkParams NetworkParams,
	loadParams LoadParams,
) (*ContLoadGenerator, error) {
	if len(flowClients) == 0 {
		return nil, errors.New("no flow clients available")
	}

	flowClient := flowClients[0]

	sc := systemcontracts.SystemContractsForChain(networkParams.ChainId)

	privateKey, err := crypto.DecodePrivateKeyHex(unittest.ServiceAccountPrivateKey.SignAlgo, networkParams.ServAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error while decoding serice account private key hex: %w", err)
	}

	servAcc, err := account.LoadAccount(ctx, flowClient, flowsdk.BytesToAddress(sc.FlowServiceAccount.Address.Bytes()), privateKey, unittest.ServiceAccountPrivateKey.HashAlgo)
	if err != nil {
		return nil, fmt.Errorf("error loading service account %w", err)
	}

	var follower TxFollower
	if loadParams.FeedbackEnabled {
		follower, err = NewTxFollower(ctx, flowClient, WithLogger(log), WithMetrics(loaderMetrics))
	} else {
		follower, err = NewNopTxFollower(ctx, flowClient)
	}
	if err != nil {
		return nil, err
	}

	lg := &ContLoadGenerator{
		ctx:                ctx,
		log:                log,
		loaderMetrics:      loaderMetrics,
		loadParams:         loadParams,
		flowClient:         flowClient,
		workerStatsTracker: workerStatsTracker,
		follower:           follower,
		stoppedChannel:     make(chan struct{}),
	}

	lg.log.Info().Int("num_keys", servAcc.NumKeys()).Msg("service account loaded")

	ts := &transactionSender{
		ctx:                      ctx,
		log:                      log,
		flowClient:               flowClient,
		loaderMetrics:            loaderMetrics,
		workerStatsTracker:       workerStatsTracker,
		follower:                 follower,
		lostTransactionThreshold: lostTransactionThreshold,
	}

	accountLoader := account.NewClientAccountLoader(lg.log, ctx, flowClient)

	err = account.EnsureAccountHasKeys(lg.log, servAcc, 50, lg.follower, ts)
	if err != nil {
		return nil, fmt.Errorf("error ensuring service account has keys: %w", err)
	}

	// we need to wait for the tx adding keys to be sealed otherwise the client won't
	// pickup the changes
	// TODO: add a better way to wait for txs to be sealed
	time.Sleep(10 * time.Second)

	err = account.ReloadAccount(accountLoader, servAcc)
	if err != nil {
		return nil, fmt.Errorf("error reloading service account: %w", err)
	}

	ap, err := account.SetupProvider(
		lg.log,
		ctx,
		loadParams.NumberOfAccounts,
		100_000_000_000,
		lg.follower,
		servAcc,
		ts,
		networkParams.ChainId.Chain(),
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up account provider: %w", err)
	}

	lc := load.LoadContext{
		ChainID: networkParams.ChainId,
		WorkerContext: load.WorkerContext{
			WorkerID: -1,
		},
		AccountProvider:        ap,
		TransactionSender:      ts,
		ReferenceBlockProvider: lg.follower,
		Proposer:               servAcc,
	}

	l := load.CreateLoadType(log, loadParams.LoadType)

	err = l.Setup(log, lc)
	if err != nil {
		return nil, fmt.Errorf("error setting up load: %w", err)
	}

	lg.workFunc = func(workerID int) {

		wlc := lc
		wlc.WorkerContext.WorkerID = workerID

		log := lg.log.With().Int("workerID", workerID).Logger()
		err := l.Load(log, wlc)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error().Err(err).Msg("error running load")
		}
	}

	return lg, nil
}

func (lg *ContLoadGenerator) stopped() bool {
	select {
	case <-lg.stoppedChannel:
		return true
	default:
		return false
	}
}

func (lg *ContLoadGenerator) startWorkers(num int) error {
	for i := 0; i < num; i++ {
		worker := NewWorker(lg.ctx, len(lg.workers), 1*time.Second, lg.workFunc)
		lg.log.Trace().Int("workerID", worker.workerID).Msg("starting worker")
		worker.Start()
		lg.workers = append(lg.workers, worker)
	}
	lg.workerStatsTracker.AddWorkers(num)
	return nil
}

func (lg *ContLoadGenerator) stopWorkers(num int) error {
	if num > len(lg.workers) {
		return fmt.Errorf("can't stop %d workers, only %d available", num, len(lg.workers))
	}

	idx := len(lg.workers) - num
	toRemove := lg.workers[idx:]
	lg.workers = lg.workers[:idx]

	for _, w := range toRemove {
		go func(w *Worker) {
			lg.log.Trace().Int("workerID", w.workerID).Msg("stopping worker")
			w.Stop()
		}(w)
	}
	lg.workerStatsTracker.AddWorkers(-num)

	return nil
}

// SetTPS compares the given TPS to the current TPS to determine whether to increase
// or decrease the load.
// It increases/decreases the load by adjusting the number of workers, since each worker
// is responsible for sending the load at 1 TPS.
func (lg *ContLoadGenerator) SetTPS(desired uint) error {
	lg.workersMutex.Lock()
	defer lg.workersMutex.Unlock()

	if lg.stopped() {
		return fmt.Errorf("SetTPS called after loader is stopped: %w", context.Canceled)
	}

	return lg.unsafeSetTPS(desired)
}

func (lg *ContLoadGenerator) unsafeSetTPS(desired uint) error {
	currentTPS := len(lg.workers)
	diff := int(desired) - currentTPS

	var err error
	switch {
	case diff > 0:
		err = lg.startWorkers(diff)
	case diff < 0:
		err = lg.stopWorkers(-diff)
	}

	if err == nil {
		lg.loaderMetrics.SetTPSConfigured(desired)
	}
	return err
}

func (lg *ContLoadGenerator) Stop() {
	lg.workersMutex.Lock()
	defer lg.workersMutex.Unlock()

	if lg.stopped() {
		lg.log.Warn().Msg("Stop() called on generator when already stopped")
		return
	}

	defer lg.log.Debug().Msg("stopped generator")

	lg.log.Debug().Msg("stopping follower")
	lg.follower.Stop()
	lg.log.Debug().Msg("stopping workers")
	_ = lg.unsafeSetTPS(0)
	close(lg.stoppedChannel)
}

func (lg *ContLoadGenerator) Done() <-chan struct{} {
	return lg.stoppedChannel
}

type transactionSender struct {
	ctx                      context.Context
	log                      zerolog.Logger
	flowClient               access.Client
	loaderMetrics            *metrics.LoaderCollector
	workerStatsTracker       *WorkerStatsTracker
	follower                 TxFollower
	workerID                 int
	lostTransactionThreshold time.Duration
}

func (t *transactionSender) Send(tx *flowsdk.Transaction) (flowsdk.TransactionResult, error) {
	// Add follower before sending the transaction to avoid race condition
	ch := t.follower.Follow(tx.ID())

	err := t.flowClient.SendTransaction(t.ctx, *tx)
	if err != nil {
		return flowsdk.TransactionResult{}, fmt.Errorf("error sending transaction: %w", err)
	}

	t.workerStatsTracker.IncTxSent()
	t.loaderMetrics.TransactionSent()

	timer := time.NewTimer(t.lostTransactionThreshold)
	defer timer.Stop()
	startTime := time.Now()

	select {
	case <-t.ctx.Done():
		return flowsdk.TransactionResult{}, t.ctx.Err()
	case result := <-ch:
		t.workerStatsTracker.IncTxExecuted()

		if result.Error != nil {
			t.workerStatsTracker.IncTxFailed()
			return result, common.NewTransactionError(result.Error)
		}

		return result, nil
	case <-timer.C:
		t.loaderMetrics.TransactionLost()
		t.log.Warn().
			Dur("duration", time.Since(startTime)).
			Msg("transaction lost")
		t.workerStatsTracker.IncTxTimedOut()
		return flowsdk.TransactionResult{}, fmt.Errorf("transaction lost")
	}
}

var _ common.TransactionSender = (*transactionSender)(nil)
