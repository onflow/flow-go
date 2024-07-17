package debug

import (
	"encoding/json"
	"fmt"

	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/eth/tracers"
	"github.com/rs/zerolog"

	// this import is needed for side-effects, because the
	// tracers.DefaultDirectory is relying on the init function
	_ "github.com/onflow/go-ethereum/eth/tracers/native"

	"github.com/onflow/flow-go/model/flow"
)

const (
	tracerConfig = `{ "onlyTopCall": true }`
	tracerName   = "callTracer"
)

type EVMTracer interface {
	WithBlockID(identifier flow.Identifier)
	TxTracer() *tracers.Tracer
	Collect(txID gethCommon.Hash)
}

var _ EVMTracer = &CallTracer{}

type CallTracer struct {
	logger   zerolog.Logger
	tracer   *tracers.Tracer
	uploader Uploader
	blockID  flow.Identifier
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

func (t *CallTracer) TxTracer() *tracers.Tracer {
	return t.tracer
}

func (t *CallTracer) WithBlockID(id flow.Identifier) {
	t.blockID = id
}

func (t *CallTracer) Collect(txID gethCommon.Hash) {
	// upload is concurrent and it doesn't produce any errors, as the
	// client doesn't expect it, we don't want to break execution flow,
	// in case there are errors we retry, and if we fail after retries
	// we log them and continue.
	go func() {
		l := t.logger.With().
			Str("tx-id", txID.String()).
			Str("block-id", t.blockID.String()).
			Logger()

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

		if err = t.uploader.Upload(TraceID(txID, t.blockID), res); err != nil {
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

func (n nopTracer) TxTracer() *tracers.Tracer {
	return nil
}

func (n nopTracer) WithBlockID(identifier flow.Identifier) {}

func (n nopTracer) Collect(_ gethCommon.Hash) {}

func TraceID(txID gethCommon.Hash, blockID flow.Identifier) string {
	return fmt.Sprintf("%s-%s", blockID.String(), txID.String())
}
