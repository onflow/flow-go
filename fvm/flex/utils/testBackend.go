package utils

import (
	"encoding/binary"
	"math"
	"math/big"
	"testing"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	gethCommon "github.com/ethereum/go-ethereum/common"
	fvmenv "github.com/onflow/flow-go/fvm/environment"
	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

func RunWithTestBackend(t testing.TB, f func(models.Backend)) {
	tb := testBackend{
		testValueStore:   getSimpleValueStore(),
		testEventEmitter: getSimpleEventEmitter(),
	}
	f(tb)
}

func RunWithEOATestAccount(t *testing.T, backend models.Backend, f func(*EOATestAccount)) {
	account := GetTestEOAAccount(t, EOATestAccount1KeyHex)

	// fund account
	db := storage.NewDatabase(backend)
	config := env.NewFlexConfig(env.WithBlockNumber(env.BlockNumberForEVMRules))

	env, err := env.NewEnvironment(config, db)
	require.NoError(t, err)

	err = env.MintTo(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)), account.FlexAddress().ToCommon())
	require.NoError(t, err)
	require.False(t, env.Result.Failed)
	f(account)
}

func RunWithDeployedContract(t *testing.T, backend models.Backend, f func(*TestContract)) {
	tc := GetTestContract(t)
	// deploy contract
	db := storage.NewDatabase(backend)
	config := env.NewFlexConfig(env.WithBlockNumber(env.BlockNumberForEVMRules))

	e, err := env.NewEnvironment(config, db)
	require.NoError(t, err)

	caller := gethCommon.Address{}
	err = e.MintTo(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)), caller)
	require.NoError(t, err)
	require.False(t, e.Result.Failed)

	e, err = env.NewEnvironment(config, db)
	require.NoError(t, err)

	err = e.Deploy(gethCommon.Address{}, tc.ByteCode, math.MaxUint64, big.NewInt(0))
	require.NoError(t, err)
	require.False(t, e.Result.Failed)

	tc.SetDeployedAt(e.Result.DeployedContractAddress)
	f(tc)
}

func ConvertToCadence(data []byte) []cadence.Value {
	ret := make([]cadence.Value, 20)
	for i, v := range data {
		ret[i] = cadence.UInt8(v)
	}
	return ret
}

func fullKey(owner, key []byte) string {
	return string(owner) + "~" + string(key)
}

func getSimpleValueStore() *testValueStore {
	data := make(map[string][]byte)
	allocator := make(map[string]uint64)

	return &testValueStore{
		getValue: func(owner, key []byte) ([]byte, error) {
			return data[fullKey(owner, key)], nil
		},
		setValue: func(owner, key, value []byte) error {
			data[fullKey(owner, key)] = value
			return nil
		},
		valueExists: func(owner, key []byte) (bool, error) {
			return len(data[fullKey(owner, key)]) > 0, nil

		},
		allocateStorageIndex: func(owner []byte) (atree.StorageIndex, error) {
			index := allocator[string(owner)]
			var data [8]byte
			allocator[string(owner)] = index + 1
			binary.BigEndian.PutUint64(data[:], index)
			return atree.StorageIndex(data), nil
		},
	}
}

func getSimpleEventEmitter() *testEventEmitter {
	events := make(flow.EventsList, 0)
	return &testEventEmitter{
		emitFlowEvent: func(etype flow.EventType, payload []byte) error {
			events = append(events, flow.Event{Type: etype, Payload: payload})
			return nil
		},
		events: func() flow.EventsList {
			return events
		},
	}
}

type testBackend struct {
	*testValueStore
	*testMeter
	*testEventEmitter
}

type testValueStore struct {
	getValue             func(owner, key []byte) ([]byte, error)
	setValue             func(owner, key, value []byte) error
	valueExists          func(owner, key []byte) (bool, error)
	allocateStorageIndex func(owner []byte) (atree.StorageIndex, error)
}

var _ fvmenv.ValueStore = &testValueStore{}

func (vs *testValueStore) GetValue(owner, key []byte) ([]byte, error) {
	if vs.getValue == nil {
		panic("method not set")
	}
	return vs.getValue(owner, key)
}

func (vs *testValueStore) SetValue(owner, key, value []byte) error {
	if vs.setValue == nil {
		panic("method not set")
	}
	return vs.setValue(owner, key, value)
}

func (vs *testValueStore) ValueExists(owner, key []byte) (bool, error) {
	if vs.valueExists == nil {
		panic("method not set")
	}
	return vs.valueExists(owner, key)
}

func (vs *testValueStore) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	if vs.allocateStorageIndex == nil {
		panic("method not set")
	}
	return vs.allocateStorageIndex(owner)
}

type testMeter struct {
	meterComputation       func(common.ComputationKind, uint) error
	computationUsed        func() (uint64, error)
	computationIntensities func() meter.MeteredComputationIntensities

	meterMemory func(usage common.MemoryUsage) error
	memoryUsed  func() (uint64, error)

	meterEmittedEvent      func(byteSize uint64) error
	totalEmittedEventBytes func() uint64

	interactionUsed func() (uint64, error)
}

var _ fvmenv.Meter = &testMeter{}

func (m *testMeter) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	if m.meterComputation == nil {
		panic("method not set")
	}
	return m.meterComputation(kind, intensity)
}

func (m *testMeter) ComputationIntensities() meter.MeteredComputationIntensities {
	if m.computationIntensities == nil {
		panic("method not set")
	}
	return m.computationIntensities()
}

func (m *testMeter) ComputationUsed() (uint64, error) {
	if m.computationUsed == nil {
		panic("method not set")
	}
	return m.computationUsed()
}

func (m *testMeter) MeterMemory(usage common.MemoryUsage) error {
	if m.meterMemory == nil {
		panic("method not set")
	}
	return m.meterMemory(usage)
}

func (m *testMeter) MemoryUsed() (uint64, error) {
	if m.memoryUsed == nil {
		panic("method not set")
	}
	return m.memoryUsed()
}

func (m *testMeter) InteractionUsed() (uint64, error) {
	if m.interactionUsed == nil {
		panic("method not set")
	}
	return m.interactionUsed()
}

func (m *testMeter) MeterEmittedEvent(byteSize uint64) error {
	if m.meterEmittedEvent == nil {
		panic("method not set")
	}
	return m.meterEmittedEvent(byteSize)
}

func (m *testMeter) TotalEmittedEventBytes() uint64 {
	if m.totalEmittedEventBytes == nil {
		panic("method not set")
	}
	return m.totalEmittedEventBytes()
}

type testEventEmitter struct {
	emitEvent              func(event cadence.Event) error
	emitFlowEvent          func(etype flow.EventType, payload []byte) error
	events                 func() flow.EventsList
	serviceEvents          func() flow.EventsList
	convertedServiceEvents func() flow.ServiceEventList
	reset                  func()
}

var _ fvmenv.EventEmitter = &testEventEmitter{}

func (vs *testEventEmitter) EmitEvent(event cadence.Event) error {
	if vs.emitEvent == nil {
		panic("method not set")
	}
	return vs.emitEvent(event)
}

func (vs *testEventEmitter) EmitFlowEvent(etype flow.EventType, payload []byte) error {
	if vs.emitFlowEvent == nil {
		panic("method not set")
	}
	return vs.emitFlowEvent(etype, payload)
}

func (vs *testEventEmitter) Events() flow.EventsList {
	if vs.events == nil {
		panic("method not set")
	}
	return vs.events()
}

func (vs *testEventEmitter) ServiceEvents() flow.EventsList {
	if vs.serviceEvents == nil {
		panic("method not set")
	}
	return vs.serviceEvents()
}

func (vs *testEventEmitter) ConvertedServiceEvents() flow.ServiceEventList {
	if vs.convertedServiceEvents == nil {
		panic("method not set")
	}
	return vs.convertedServiceEvents()
}

func (vs *testEventEmitter) Reset() {
	if vs.reset == nil {
		panic("method not set")
	}
	vs.reset()
}
