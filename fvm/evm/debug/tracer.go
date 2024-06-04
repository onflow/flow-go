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

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

const (
	initialTimeout = 2 * time.Second
	maxRetryNumber = 10
	tracerConfig   = `{ "onlyTopCall": true }`
	tracerName     = "callTracer"
)

type TracerManager struct {
	logger    zerolog.Logger
	traces    map[uint64]map[flow.Identifier][]json.RawMessage
	traceChan chan *CallTracer
	uploader  Uploader
	headers   storage.Headers
}

func NewTracerManager(uploader Uploader, logger zerolog.Logger) *TracerManager {
	const bufferSize = 128 // todo revisit

	return &TracerManager{
		logger:    logger.With().Str("module", "evm-tracer").Logger(),
		traces:    make(map[uint64]map[flow.Identifier][]json.RawMessage),
		traceChan: make(chan *CallTracer, bufferSize),
		uploader:  uploader,
	}
}

func (t *TracerManager) NewTracer(header *flow.Header) (*CallTracer, error) {
	tracer, err := tracers.DefaultDirectory.New(tracerName, &tracers.Context{}, json.RawMessage(tracerConfig))
	if err != nil {
		return nil, err
	}

	return &CallTracer{
		Tracer: tracer,
		header: header,
	}, nil
}

// OnFinalizedBlock handles uploading of all the EVM traces that were produced
// in this block, identified by ID.
//
// Because there is optimistic execution and possible forks we can have multiple
// blocks at the same block height in the queue (trace chanell),
// each contains EVM traces identified by the same EVM transaction ID.
// We must wait for block to be sealed and at that point we go through the queue
// of traces, make sure we only update traces from the block ID we finalized,
// and discard all other traces that were produced at the same block height.
// In case there are some traces already in the queue for the next block height
// we requeue them for later processing, once that block height is sealed.
func (t *TracerManager) OnFinalizedBlock(block *model.Block) {
	l := t.logger.With().Str("block-id", block.BlockID.String()).Logger()

	l.Debug().Msg("received new finalized block event, upload traces")

	header, err := t.headers.ByBlockID(block.BlockID)
	if err != nil {
		l.Error().Err(err).Msg("failed to get block height")
	}

	for trace := range t.traceChan {
		lt := l.With().Str("tx-id", trace.evmID.String()).Logger()

		if trace.header.ID() == header.ID() {
			result, err := trace.GetResult()
			if err != nil {
				lt.Error().Err(err).Msg("failed to get trace result")
			}

			if err = t.uploader.Upload(trace.evmID.String(), result); err != nil {
				lt.Error().Err(err).Msg("failed to upload trace")
				continue
			}

			lt.Info().Msg("trace uploaded")
		}

		if trace.header.Height != header.Height {
			t.traceChan <- trace
		}
	}
}

func (t *TracerManager) Collect(id gethCommon.Hash, tracer *CallTracer) {
	l := t.logger.With().Str("tx-id", id.String()).Logger()

	// make sure we catch all panics so we don't affect the execution flow
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

	// todo improve / move
	tracer.evmID = id

	select {
	case t.traceChan <- tracer:
		l.Debug().Msg("trace pushed to the queue")
	default:
		result, err := tracer.GetResult()
		if err != nil {
			l.Error().Err(err).Msg("failed to get trace result and failed to push the trace to the queue")
			return
		}
		l.Error().Str("trace", string(result)).Msg("failed to push trace to the queue")
	}
}

type CallTracer struct {
	tracers.Tracer
	evmID  gethCommon.Hash
	header *flow.Header
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
