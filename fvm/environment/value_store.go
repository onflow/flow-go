package environment

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/atree"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// ValueStore provides read/write access to the account storage.
type ValueStore interface {
	GetValue(owner []byte, key []byte) ([]byte, error)

	SetValue(owner, key, value []byte) error

	ValueExists(owner []byte, key []byte) (bool, error)

	AllocateStorageIndex(owner []byte) (atree.StorageIndex, error)
}

type ParseRestrictedValueStore struct {
	txnState *state.TransactionState
	impl     ValueStore
}

func NewParseRestrictedValueStore(
	txnState *state.TransactionState,
	impl ValueStore,
) ValueStore {
	return ParseRestrictedValueStore{
		txnState: txnState,
		impl:     impl,
	}
}

func (store ParseRestrictedValueStore) GetValue(
	owner []byte,
	key []byte,
) (
	[]byte,
	error,
) {
	return parseRestrict2Arg1Ret(
		store.txnState,
		"GetValue",
		store.impl.GetValue,
		owner,
		key)
}

func (store ParseRestrictedValueStore) SetValue(
	owner []byte,
	key []byte,
	value []byte,
) error {
	return parseRestrict3Arg(
		store.txnState,
		"SetValue",
		store.impl.SetValue,
		owner,
		key,
		value)
}

func (store ParseRestrictedValueStore) ValueExists(
	owner []byte,
	key []byte,
) (
	bool,
	error,
) {
	return parseRestrict2Arg1Ret(
		store.txnState,
		"ValueExists",
		store.impl.ValueExists,
		owner,
		key)
}

func (store ParseRestrictedValueStore) AllocateStorageIndex(
	owner []byte,
) (
	atree.StorageIndex,
	error,
) {
	return parseRestrict1Arg1Ret(
		store.txnState,
		"AllocateStorageIndex",
		store.impl.AllocateStorageIndex,
		owner)
}

type valueStore struct {
	tracer *Tracer
	meter  Meter

	accounts Accounts
}

func NewValueStore(tracer *Tracer, meter Meter, accounts Accounts) ValueStore {
	return &valueStore{
		tracer:   tracer,
		meter:    meter,
		accounts: accounts,
	}
}

func (store *valueStore) GetValue(owner []byte, key []byte) ([]byte, error) {
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
func (store *valueStore) SetValue(
	owner []byte,
	key []byte,
	value []byte,
) error {
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

func (store *valueStore) ValueExists(
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

// AllocateStorageIndex allocates new storage index under the owner accounts
// to store a new register.
func (store *valueStore) AllocateStorageIndex(
	owner []byte,
) (
	atree.StorageIndex,
	error,
) {
	err := store.meter.MeterComputation(ComputationKindAllocateStorageIndex, 1)
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf(
			"allocate storage index failed: %w",
			err)
	}

	v, err := store.accounts.AllocateStorageIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf(
			"storage address allocation failed: %w",
			err)
	}
	return v, nil
}
