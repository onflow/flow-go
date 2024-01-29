package environment

import (
	"fmt"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// ValueStore provides read/write access to the account storage.
type ValueStore interface {
	GetValue(owner []byte, key []byte) ([]byte, error)

	SetValue(owner, key, value []byte) error

	ValueExists(owner []byte, key []byte) (bool, error)

	AllocateSlabIndex(owner []byte) (atree.SlabIndex, error)
}

type ParseRestrictedValueStore struct {
	txnState state.NestedTransactionPreparer
	impl     ValueStore
}

func NewParseRestrictedValueStore(
	txnState state.NestedTransactionPreparer,
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
		trace.FVMEnvGetValue,
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
		trace.FVMEnvSetValue,
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
		trace.FVMEnvValueExists,
		store.impl.ValueExists,
		owner,
		key)
}

func (store ParseRestrictedValueStore) AllocateSlabIndex(
	owner []byte,
) (
	atree.SlabIndex,
	error,
) {
	return parseRestrict1Arg1Ret(
		store.txnState,
		trace.FVMEnvAllocateStorageIndex,
		store.impl.AllocateSlabIndex,
		owner)
}

type valueStore struct {
	tracer tracing.TracerSpan
	meter  Meter

	accounts Accounts
}

func NewValueStore(
	tracer tracing.TracerSpan,
	meter Meter,
	accounts Accounts,
) ValueStore {
	return &valueStore{
		tracer:   tracer,
		meter:    meter,
		accounts: accounts,
	}
}

func (store *valueStore) GetValue(
	owner []byte,
	keyBytes []byte,
) (
	[]byte,
	error,
) {
	defer store.tracer.StartChildSpan(trace.FVMEnvGetValue).End()

	id := flow.CadenceRegisterID(owner, keyBytes)
	if id.IsInternalState() {
		return nil, errors.NewInvalidInternalStateAccessError(id, "read")
	}

	v, err := store.accounts.GetValue(id)
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}

	err = store.meter.MeterComputation(ComputationKindGetValue, uint(len(v)))
	if err != nil {
		return nil, fmt.Errorf("get value failed: %w", err)
	}
	return v, nil
}

// TODO disable SetValue for scripts, right now the view changes are discarded
func (store *valueStore) SetValue(
	owner []byte,
	keyBytes []byte,
	value []byte,
) error {
	defer store.tracer.StartChildSpan(trace.FVMEnvSetValue).End()

	id := flow.CadenceRegisterID(owner, keyBytes)
	if id.IsInternalState() {
		return errors.NewInvalidInternalStateAccessError(id, "modify")
	}

	err := store.meter.MeterComputation(
		ComputationKindSetValue,
		uint(len(value)))
	if err != nil {
		return fmt.Errorf("set value failed: %w", err)
	}

	err = store.accounts.SetValue(id, value)
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
	defer store.tracer.StartChildSpan(trace.FVMEnvValueExists).End()

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

// AllocateSlabIndex allocates new storage index under the owner accounts
// to store a new register.
func (store *valueStore) AllocateSlabIndex(
	owner []byte,
) (
	atree.SlabIndex,
	error,
) {
	defer store.tracer.StartChildSpan(trace.FVMEnvAllocateStorageIndex).End()

	err := store.meter.MeterComputation(ComputationKindAllocateStorageIndex, 1)
	if err != nil {
		return atree.SlabIndex{}, fmt.Errorf(
			"allocate storage index failed: %w",
			err)
	}

	v, err := store.accounts.AllocateSlabIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.SlabIndex{}, fmt.Errorf(
			"storage address allocation failed: %w",
			err)
	}
	return v, nil
}
