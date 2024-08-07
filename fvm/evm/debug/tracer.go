package debug

import (
	"encoding/json"
	"fmt"
	"math/big"

	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/core/tracing"
	"github.com/onflow/go-ethereum/core/types"
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
	txTracer *tracers.Tracer
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
	if t.txTracer == nil {
		t.txTracer = NewSafeTxTracer(t)
	}
	return t.txTracer
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

func NewSafeTxTracer(ct *CallTracer) *tracers.Tracer {
	wrapped := &tracers.Tracer{
		Hooks: &tracing.Hooks{},
	}

	l := ct.logger.With().
		Str("block-id", ct.blockID.String()).
		Logger()

	if ct.tracer.OnTxStart != nil {
		wrapped.OnTxStart = func(
			vm *tracing.VMContext,
			tx *types.Transaction,
			from gethCommon.Address,
		) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("panic: %v", r)
					}
					l.Err(err).
						Stack().
						Msg("OnTxStart trace collection failed")
				}
			}()
			ct.tracer.OnTxStart(vm, tx, from)
		}
	}

	if ct.tracer.OnTxEnd != nil {
		wrapped.OnTxEnd = func(receipt *types.Receipt, err error) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("panic: %v", r)
					}
					l.Err(err).
						Stack().
						Msg("OnTxEnd trace collection failed")
				}
			}()
			ct.tracer.OnTxEnd(receipt, err)
		}
	}

	if ct.tracer.OnEnter != nil {
		wrapped.OnEnter = func(
			depth int,
			typ byte,
			from, to gethCommon.Address,
			input []byte,
			gas uint64,
			value *big.Int,
		) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("panic: %v", r)
					}
					l.Err(err).
						Stack().
						Msg("OnEnter trace collection failed")
				}
			}()
			ct.tracer.OnEnter(depth, typ, from, to, input, gas, value)
		}
	}

	if ct.tracer.OnExit != nil {
		wrapped.OnExit = func(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("panic: %v", r)
					}
					l.Err(err).
						Stack().
						Msg("OnExit trace collection failed")
				}
			}()
			ct.tracer.OnExit(depth, output, gasUsed, err, reverted)
		}
	}

	if ct.tracer.OnOpcode != nil {
		wrapped.OnOpcode = func(
			pc uint64,
			op byte,
			gas, cost uint64,
			scope tracing.OpContext,
			rData []byte,
			depth int,
			err error,
		) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("panic: %v", r)
					}
					l.Err(err).
						Stack().
						Msg("OnOpcode trace collection failed")
				}
			}()
			ct.tracer.OnOpcode(pc, op, gas, cost, scope, rData, depth, err)
		}
	}

	if ct.tracer.OnFault != nil {
		wrapped.OnFault = func(
			pc uint64,
			op byte,
			gas, cost uint64,
			scope tracing.OpContext,
			depth int,
			err error) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("panic: %v", r)
					}
					l.Err(err).
						Stack().
						Msg("OnFault trace collection failed")
				}
			}()
			ct.tracer.OnFault(pc, op, gas, cost, scope, depth, err)
		}
	}

	if ct.tracer.OnGasChange != nil {
		wrapped.OnGasChange = func(old, new uint64, reason tracing.GasChangeReason) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("panic: %v", r)
					}
					l.Err(err).
						Stack().
						Msg("OnGasChange trace collection failed")
				}
			}()
			ct.tracer.OnGasChange(old, new, reason)
		}
	}

	wrapped.GetResult = ct.tracer.GetResult
	wrapped.Stop = ct.tracer.Stop
	return wrapped
}
