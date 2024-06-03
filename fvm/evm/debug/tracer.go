package debug

import (
	"encoding/json"

	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/eth/tracers"
	"github.com/rs/zerolog"

	// this import is needed for side-effects, because the
	// tracers.DefaultDirectory is relying on the init function
	_ "github.com/onflow/go-ethereum/eth/tracers/native"
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
	tracerType := json.RawMessage(`{ "onlyTopCall": true }`)
	tracer, err := tracers.DefaultDirectory.New("callTracer", &tracers.Context{}, tracerType)
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

		res, err := t.tracer.GetResult()
		if err != nil {
			l.Error().Err(err).Msg("failed to produce trace results")
		}

		err = t.uploader.Upload(id.String(), res)
		if err != nil {
			l.Error().Err(err).Msg("failed to upload trace results")
		}
	}()
}

var NopTracer = &nopTracer{}

var _ EVMTracer = &nopTracer{}

type nopTracer struct{}

func (n nopTracer) TxTracer() tracers.Tracer {
	return nil
}

func (n nopTracer) Collect(_ gethCommon.Hash) {}
