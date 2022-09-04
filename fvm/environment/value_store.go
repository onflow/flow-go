package environment

import (
	"encoding/hex"
	"fmt"

	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// ValueStore provides read/write access to the account storage.
type ValueStore struct {
	tracer *Tracer
	meter  Meter

	accounts Accounts
}

func NewValueStore(tracer *Tracer, meter Meter, accounts Accounts) *ValueStore {
	return &ValueStore{
		tracer:   tracer,
		meter:    meter,
		accounts: accounts,
	}
}

func (store *ValueStore) GetValue(owner, key []byte) ([]byte, error) {
	var valueByteSize int
	span := store.tracer.StartSpanFromRoot(trace.FVMEnvGetValue)
	defer func() {
		if !trace.IsSampled(span) {
			span.SetAttributes(
				attribute.String("owner", hex.EncodeToString(owner)),
				attribute.String("key", string(key)),
				attribute.Int("valueByteSize", valueByteSize),
			)
		}
		span.End()
	}()

	v, err := store.accounts.GetValue(
		flow.BytesToAddress(owner),
		string(key),
	)
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}
	valueByteSize = len(v)

	err = store.meter.MeterComputation(
		ComputationKindGetValue,
		uint(valueByteSize))
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}
	return v, nil
}

// TODO disable SetValue for scripts, right now the view changes are discarded
func (store *ValueStore) SetValue(owner, key, value []byte) error {
	span := store.tracer.StartSpanFromRoot(trace.FVMEnvSetValue)
	if !trace.IsSampled(span) {
		span.SetAttributes(
			attribute.String("owner", hex.EncodeToString(owner)),
			attribute.String("key", string(key)),
		)
	}
	defer span.End()

	err := store.meter.MeterComputation(
		ComputationKindSetValue,
		uint(len(value)))
	if err != nil {
		return fmt.Errorf("set value failed: %w", err)
	}

	err = store.accounts.SetValue(
		flow.BytesToAddress(owner),
		string(key),
		value,
	)
	if err != nil {
		return fmt.Errorf("set value failed: %w", err)
	}
	return nil
}

func (store *ValueStore) ValueExists(
	owner []byte,
	key []byte,
) (
	exists bool,
	err error,
) {
	defer store.tracer.StartSpanFromRoot(trace.FVMEnvValueExists).End()

	err = store.meter.MeterComputation(ComputationKindValueExists, 1)
	if err != nil {
		return false, fmt.Errorf("check value existence failed: %w", err)
	}

	v, err := store.GetValue(owner, key)
	if err != nil {
		return false, fmt.Errorf("check value existence failed: %w", err)
	}

	return len(v) > 0, nil
}
