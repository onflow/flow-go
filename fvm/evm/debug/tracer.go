package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/eth/tracers"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	// this import is needed for side-effects, because the
	// tracers.DefaultDirectory is relying on the init function
	_ "github.com/onflow/go-ethereum/eth/tracers/native"
)

const (
	initialTimeout = 2 * time.Second
	maxRetryNumber = 10
	tracerConfig   = `{ "onlyTopCall": true }`
	tracerName     = "callTracer"
)

type EVMTracer interface {
	TxTracer() tracers.Tracer
	Collect(id gethCommon.Hash)
}

var _ EVMTracer = &CallTracer{}

type CallTracer struct {
	logger   zerolog.Logger
	tracer   tracers.Tracer
	uploader Uploader
}

func NewEVMCallTracer(uploader Uploader, logger zerolog.Logger) (*CallTracer, error) {
	tracer, err := tracers.DefaultDirectory.New(tracerName, &tracers.Context{}, json.RawMessage(tracerConfig))
	if err != nil {
		return nil, err
	}

	return &CallTracer{
		logger:   logger.With().Str("module", "evm-tracer").Logger(),
		tracer:   tracer,
		uploader: uploader,
	}, nil
}

func (t *CallTracer) TxTracer() tracers.Tracer {
	return t.tracer
}

func (t *CallTracer) Collect(id gethCommon.Hash) {
	// upload is concurrent and it doesn't produce any errors, as the
	// client doesn't expect it, we don't want to break execution flow,
	// in case there are errors we retry, and if we fail after retries
	// we log them and continue.
	go func() {
		l := t.logger.With().Str("tx-id", id.String()).Logger()

		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("panic: %v", r)
				}

				l.Err(err).
					Stack().
					Msg("failed to collect EVM traces")
			}
		}()

		res, err := t.tracer.GetResult()
		if err != nil {
			l.Error().Err(err).Msg("failed to produce trace results")
		}

		backoff := retry.NewFibonacci(initialTimeout)
		backoff = retry.WithMaxRetries(maxRetryNumber, backoff)

		err = retry.Do(context.Background(), backoff, func(ctx context.Context) error {
			err = t.uploader.Upload(id.String(), res)
			if err != nil {
				l.Error().Err(err).Msg("failed to upload trace results, retrying")
				err = retry.RetryableError(err) // all upload errors should be retried
			}
			return err
		})
		if err != nil {
			l.Error().Err(err).
				Str("traces", string(res)).
				Msg("failed to upload trace results, no more retries")
			return
		}

		l.Debug().Msg("evm traces uploaded successfully")
	}()
}

var NopTracer = &nopTracer{}

var _ EVMTracer = &nopTracer{}

type nopTracer struct{}

func (n nopTracer) TxTracer() tracers.Tracer {
	return nil
}

func (n nopTracer) Collect(_ gethCommon.Hash) {}
