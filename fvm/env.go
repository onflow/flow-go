package fvm

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/opentracing/opentracing-go"
	traceLog "github.com/opentracing/opentracing-go/log"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// Environment accepts a context and a virtual machine instance and provides
// cadence runtime interface methods to the runtime.
type Environment interface {
	Context() *Context
	VM() *VirtualMachine
	runtime.Interface
}

// TODO(patrick): refactor this into an object after merging #2800
type ComputationMeter interface {
	Meter(common.ComputationKind, uint) error
}

// Parts of the environment that are common to all transaction and script
// executions.
type commonEnv struct {
	ComputationMeter

	ctx           Context
	sth           *state.StateHolder
	vm            *VirtualMachine
	programs      *handler.ProgramsHandler
	accounts      state.Accounts
	accountKeys   *handler.AccountKeyHandler
	contracts     *handler.ContractHandler
	uuidGenerator *state.UUIDGenerator
	metrics       *handler.MetricsHandler
	logs          []string
	rng           *rand.Rand
	traceSpan     opentracing.Span
}

func (env *commonEnv) Context() *Context {
	return &env.ctx
}

func (env *commonEnv) VM() *VirtualMachine {
	return env.vm
}

func (env *commonEnv) seedRNG(header *flow.Header) {
	// Seed the random number generator with entropy created from the block
	// header ID. The random number generator will be used by the UnsafeRandom
	// function.
	id := header.ID()
	source := rand.NewSource(int64(binary.BigEndian.Uint64(id[:])))
	env.rng = rand.New(source)
}

func (env *commonEnv) isTraceable() bool {
	return env.ctx.Tracer != nil && env.traceSpan != nil
}

func (env *commonEnv) ImplementationDebugLog(message string) error {
	env.ctx.Logger.Debug().Msgf("Cadence: %s", message)
	return nil
}

func (env *commonEnv) Hash(
	data []byte,
	tag string,
	hashAlgorithm runtime.HashAlgorithm,
) ([]byte, error) {
	if env.isTraceable() {
		sp := env.ctx.Tracer.StartSpanFromParent(env.traceSpan, trace.FVMEnvHash)
		defer sp.Finish()
	}

	err := env.Meter(meter.ComputationKindHash, 1)
	if err != nil {
		return nil, fmt.Errorf("hash failed: %w", err)
	}

	hashAlgo := crypto.RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	return crypto.HashWithTag(hashAlgo, tag, data)
}

func (env *commonEnv) RecordTrace(operation string, location common.Location, duration time.Duration, logs []opentracing.LogRecord) {
	if !env.isTraceable() {
		return
	}
	if location != nil {
		if logs == nil {
			logs = make([]opentracing.LogRecord, 0, 1)
		}
		logs = append(logs, opentracing.LogRecord{Timestamp: time.Now(),
			Fields: []traceLog.Field{traceLog.String("location", location.String())},
		})
	}
	spanName := trace.FVMCadenceTrace.Child(operation)
	env.ctx.Tracer.RecordSpanFromParent(env.traceSpan, spanName, duration, logs)
}

func (env *commonEnv) ProgramParsed(location common.Location, duration time.Duration) {
	env.RecordTrace("parseProgram", location, duration, nil)
	env.metrics.ProgramParsed(location, duration)
}

func (env *commonEnv) ProgramChecked(location common.Location, duration time.Duration) {
	env.RecordTrace("checkProgram", location, duration, nil)
	env.metrics.ProgramChecked(location, duration)
}

func (env *commonEnv) ProgramInterpreted(location common.Location, duration time.Duration) {
	env.RecordTrace("interpretProgram", location, duration, nil)
	env.metrics.ProgramInterpreted(location, duration)
}

func (env *commonEnv) ValueEncoded(duration time.Duration) {
	env.RecordTrace("encodeValue", nil, duration, nil)
	env.metrics.ValueEncoded(duration)
}

func (env *commonEnv) ValueDecoded(duration time.Duration) {
	env.RecordTrace("decodeValue", nil, duration, nil)
	env.metrics.ValueDecoded(duration)
}

// Commit commits changes and return a list of updated keys
func (env *commonEnv) Commit() ([]programs.ContractUpdateKey, error) {
	// commit changes and return a list of updated keys
	err := env.programs.Cleanup()
	if err != nil {
		return nil, err
	}
	return env.contracts.Commit()
}

func (commonEnv) BLSVerifyPOP(pk *runtime.PublicKey, sig []byte) (bool, error) {
	return crypto.VerifyPOP(pk, sig)
}

func (commonEnv) BLSAggregateSignatures(sigs [][]byte) ([]byte, error) {
	return crypto.AggregateSignatures(sigs)
}

func (commonEnv) BLSAggregatePublicKeys(
	keys []*runtime.PublicKey,
) (*runtime.PublicKey, error) {

	return crypto.AggregatePublicKeys(keys)
}

func (commonEnv) ResourceOwnerChanged(
	*interpreter.Interpreter,
	*interpreter.CompositeValue,
	common.Address,
	common.Address,
) {
}
