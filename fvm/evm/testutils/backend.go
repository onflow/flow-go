package testutils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/onflow/cadence/stdlib"
	"github.com/rs/zerolog"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

var TestFlowEVMRootAddress = flow.Address{1, 2, 3, 4}
var TestComputationLimit = uint(100_000_000)

func RunWithTestFlowEVMRootAddress(t testing.TB, backend atree.Ledger, f func(flow.Address)) {
	as := environment.NewAccountStatus()
	err := backend.SetValue(TestFlowEVMRootAddress[:], []byte(flow.AccountStatusKey), as.ToBytes())
	require.NoError(t, err)
	f(TestFlowEVMRootAddress)
}

func RunWithTestBackend(t testing.TB, f func(*TestBackend)) {
	tb := &TestBackend{
		TestValueStore:              GetSimpleValueStore(),
		testEventEmitter:            getSimpleEventEmitter(),
		testMeter:                   getSimpleMeter(),
		TestBlockInfo:               getSimpleBlockStore(),
		TestRandomGenerator:         getSimpleRandomGenerator(),
		TestContractFunctionInvoker: &TestContractFunctionInvoker{},
		TestTracer:                  &TestTracer{},
		TestMetricsReporter:         &TestMetricsReporter{},
		TestLoggerProvider:          &TestLoggerProvider{},
	}
	f(tb)
}

func ConvertToCadence(data []byte) []cadence.Value {
	ret := make([]cadence.Value, len(data))
	for i, v := range data {
		ret[i] = cadence.UInt8(v)
	}
	return ret
}

func fullKey(owner, key []byte) string {
	return fmt.Sprintf("%x~%s", owner, key)
}

func GetSimpleValueStore() *TestValueStore {
	return GetSimpleValueStorePopulated(
		make(map[string][]byte),
		make(map[string]uint64),
	)
}

func GetSimpleValueStorePopulated(
	data map[string][]byte,
	allocator map[string]uint64,
) *TestValueStore {
	bytesRead := 0
	bytesWritten := 0
	return &TestValueStore{
		GetValueFunc: func(owner, key []byte) ([]byte, error) {
			fk := fullKey(owner, key)
			value := data[fk]
			bytesRead += len(fk) + len(value)
			return value, nil
		},
		SetValueFunc: func(owner, key, value []byte) error {
			fk := fullKey(owner, key)
			data[fk] = value
			bytesWritten += len(fk) + len(value)
			return nil
		},
		ValueExistsFunc: func(owner, key []byte) (bool, error) {
			fk := fullKey(owner, key)
			value := data[fk]
			bytesRead += len(fk) + len(value)
			return len(value) > 0, nil
		},
		AllocateSlabIndexFunc: func(owner []byte) (atree.SlabIndex, error) {
			index := allocator[string(owner)]
			// TODO: figure out why it result in a collision
			if index == 0 {
				index = 10
			}
			var data [8]byte
			allocator[string(owner)] = index + 1
			binary.BigEndian.PutUint64(data[:], index)
			bytesRead += len(owner) + 8
			bytesWritten += len(owner) + 8
			return atree.SlabIndex(data), nil
		},
		TotalStorageSizeFunc: func() int {
			size := 0
			for key, item := range data {
				size += len(item) + len([]byte(key))
			}
			for key := range allocator {
				size += len(key) + 8
			}
			return size
		},
		TotalBytesReadFunc: func() int {
			return bytesRead
		},
		TotalBytesWrittenFunc: func() int {
			return bytesWritten
		},
		TotalStorageItemsFunc: func() int {
			return len(maps.Keys(data)) + len(maps.Keys(allocator))
		},
		ResetStatsFunc: func() {
			bytesRead = 0
			bytesWritten = 0
		},

		CloneFunc: func() *TestValueStore {
			// clone data
			newData := make(map[string][]byte)
			for k, v := range data {
				newData[k] = v
			}
			newAllocator := make(map[string]uint64)
			for k, v := range allocator {
				newAllocator[k] = v
			}
			// clone allocator
			return GetSimpleValueStorePopulated(newData, newAllocator)
		},

		DumpFunc: func() (map[string][]byte, map[string]uint64) {
			// clone data
			newData := make(map[string][]byte)
			for k, v := range data {
				newData[k] = v
			}
			newAllocator := make(map[string]uint64)
			for k, v := range allocator {
				newAllocator[k] = v
			}
			return newData, newAllocator
		},
	}
}

func getSimpleEventEmitter() *testEventEmitter {
	events := make(flow.EventsList, 0)
	return &testEventEmitter{
		emitEvent: func(event cadence.Event) error {
			payload, err := ccf.Encode(event)
			if err != nil {
				return err
			}
			eventType := flow.EventType(event.EventType.ID())
			events = append(events, flow.Event{
				Type:    eventType,
				Payload: payload,
			})
			return nil
		},
		events: func() flow.EventsList {
			return events
		},
		reset: func() {
			events = make(flow.EventsList, 0)
		},
	}
}

func getSimpleMeter() *testMeter {
	compUsed := uint(0)
	return &testMeter{
		meterComputation: func(kind common.ComputationKind, intensity uint) error {
			compUsed += intensity
			if compUsed > TestComputationLimit {
				return fmt.Errorf("computation limit has hit %d", TestComputationLimit)
			}
			return nil
		},
		hasComputationCapacity: func(kind common.ComputationKind, intensity uint) bool {
			return compUsed+intensity < TestComputationLimit
		},
		computationUsed: func() (uint64, error) {
			return uint64(compUsed), nil
		},
	}
}

func getSimpleBlockStore() *TestBlockInfo {
	var index int64 = 1
	return &TestBlockInfo{
		GetCurrentBlockHeightFunc: func() (uint64, error) {
			index++
			return uint64(index), nil
		},
		GetBlockAtHeightFunc: func(height uint64) (runtime.Block, bool, error) {
			return runtime.Block{
				Height:    height,
				View:      0,
				Hash:      stdlib.BlockHash{},
				Timestamp: int64(height),
			}, true, nil
		},
	}
}

type TestBackend struct {
	*TestValueStore
	*testMeter
	*testEventEmitter
	*TestBlockInfo
	*TestRandomGenerator
	*TestContractFunctionInvoker
	*testUUIDGenerator
	*TestTracer
	*TestMetricsReporter
	*TestLoggerProvider
}

var _ types.Backend = &TestBackend{}

func (tb *TestBackend) TotalStorageSize() int {
	if tb.TotalStorageSizeFunc == nil {
		panic("method not set")
	}
	return tb.TotalStorageSizeFunc()
}

func (tb *TestBackend) DropEvents() {
	if tb.reset == nil {
		panic("method not set")
	}
	tb.reset()
}

func (tb *TestBackend) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	return tb.GetValue([]byte(id.Owner), []byte(id.Key))
}

type TestValueStore struct {
	GetValueFunc          func(owner, key []byte) ([]byte, error)
	SetValueFunc          func(owner, key, value []byte) error
	ValueExistsFunc       func(owner, key []byte) (bool, error)
	AllocateSlabIndexFunc func(owner []byte) (atree.SlabIndex, error)
	TotalStorageSizeFunc  func() int
	TotalBytesReadFunc    func() int
	TotalBytesWrittenFunc func() int
	TotalStorageItemsFunc func() int
	ResetStatsFunc        func()
	CloneFunc             func() *TestValueStore
	DumpFunc              func() (map[string][]byte, map[string]uint64)
}

var _ environment.ValueStore = &TestValueStore{}

func (vs *TestValueStore) GetValue(owner, key []byte) ([]byte, error) {
	if vs.GetValueFunc == nil {
		panic("method not set")
	}
	return vs.GetValueFunc(owner, key)
}

func (vs *TestValueStore) SetValue(owner, key, value []byte) error {
	if vs.SetValueFunc == nil {
		panic("method not set")
	}
	return vs.SetValueFunc(owner, key, value)
}

func (vs *TestValueStore) ValueExists(owner, key []byte) (bool, error) {
	if vs.ValueExistsFunc == nil {
		panic("method not set")
	}
	return vs.ValueExistsFunc(owner, key)
}

func (vs *TestValueStore) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	if vs.AllocateSlabIndexFunc == nil {
		panic("method not set")
	}
	return vs.AllocateSlabIndexFunc(owner)
}

func (vs *TestValueStore) TotalBytesRead() int {
	if vs.TotalBytesReadFunc == nil {
		panic("method not set")
	}
	return vs.TotalBytesReadFunc()
}

func (vs *TestValueStore) TotalBytesWritten() int {
	if vs.TotalBytesWrittenFunc == nil {
		panic("method not set")
	}
	return vs.TotalBytesWrittenFunc()
}

func (vs *TestValueStore) TotalStorageSize() int {
	if vs.TotalStorageSizeFunc == nil {
		panic("method not set")
	}
	return vs.TotalStorageSizeFunc()
}

func (vs *TestValueStore) TotalStorageItems() int {
	if vs.TotalStorageItemsFunc == nil {
		panic("method not set")
	}
	return vs.TotalStorageItemsFunc()
}

func (vs *TestValueStore) ResetStats() {
	if vs.ResetStatsFunc == nil {
		panic("method not set")
	}
	vs.ResetStatsFunc()
}

func (vs *TestValueStore) Clone() *TestValueStore {
	if vs.CloneFunc == nil {
		panic("method not set")
	}
	return vs.CloneFunc()
}

func (vs *TestValueStore) Dump() (map[string][]byte, map[string]uint64) {
	if vs.DumpFunc == nil {
		panic("method not set")
	}
	return vs.DumpFunc()
}

type testMeter struct {
	meterComputation       func(common.ComputationKind, uint) error
	hasComputationCapacity func(common.ComputationKind, uint) bool
	computationUsed        func() (uint64, error)
	computationIntensities func() meter.MeteredComputationIntensities

	meterMemory func(usage common.MemoryUsage) error
	memoryUsed  func() (uint64, error)

	meterEmittedEvent      func(byteSize uint64) error
	totalEmittedEventBytes func() uint64

	interactionUsed func() (uint64, error)
}

var _ environment.Meter = &testMeter{}

func (m *testMeter) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	if m.meterComputation == nil {
		panic("method not set")
	}
	return m.meterComputation(kind, intensity)
}

func (m *testMeter) ComputationAvailable(
	kind common.ComputationKind,
	intensity uint,
) bool {
	if m.hasComputationCapacity == nil {
		panic("method not set")
	}
	return m.hasComputationCapacity(kind, intensity)
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
	events                 func() flow.EventsList
	serviceEvents          func() flow.EventsList
	convertedServiceEvents func() flow.ServiceEventList
	reset                  func()
}

var _ environment.EventEmitter = &testEventEmitter{}

func (vs *testEventEmitter) EmitEvent(event cadence.Event) error {
	if vs.emitEvent == nil {
		panic("method not set")
	}
	return vs.emitEvent(event)
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

type TestBlockInfo struct {
	GetCurrentBlockHeightFunc func() (uint64, error)
	GetBlockAtHeightFunc      func(height uint64) (runtime.Block, bool, error)
}

var _ environment.BlockInfo = &TestBlockInfo{}

// GetCurrentBlockHeight returns the current block height.
func (tb *TestBlockInfo) GetCurrentBlockHeight() (uint64, error) {
	if tb.GetCurrentBlockHeightFunc == nil {
		panic("GetCurrentBlockHeight method is not set")
	}
	return tb.GetCurrentBlockHeightFunc()
}

// GetBlockAtHeight returns the block at the given height.
func (tb *TestBlockInfo) GetBlockAtHeight(height uint64) (runtime.Block, bool, error) {
	if tb.GetBlockAtHeightFunc == nil {
		panic("GetBlockAtHeight method is not set")
	}
	return tb.GetBlockAtHeightFunc(height)
}

type TestRandomGenerator struct {
	ReadRandomFunc func([]byte) error
}

var _ environment.RandomGenerator = &TestRandomGenerator{}

func (t *TestRandomGenerator) ReadRandom(buffer []byte) error {
	if t.ReadRandomFunc == nil {
		panic("ReadRandomFunc method is not set")
	}
	return t.ReadRandomFunc(buffer)
}

func getSimpleRandomGenerator() *TestRandomGenerator {
	return &TestRandomGenerator{
		ReadRandomFunc: func(buffer []byte) error {
			_, err := rand.Read(buffer)
			return err
		},
	}
}

type TestContractFunctionInvoker struct {
	InvokeFunc func(
		spec environment.ContractFunctionSpec,
		arguments []cadence.Value,
	) (
		cadence.Value,
		error,
	)
}

var _ environment.ContractFunctionInvoker = &TestContractFunctionInvoker{}

func (t *TestContractFunctionInvoker) Invoke(
	spec environment.ContractFunctionSpec,
	arguments []cadence.Value,
) (
	cadence.Value,
	error,
) {
	if t.InvokeFunc == nil {
		panic("InvokeFunc method is not set")
	}
	return t.InvokeFunc(spec, arguments)
}

type testUUIDGenerator struct {
	generateUUID func() (uint64, error)
}

var _ environment.UUIDGenerator = &testUUIDGenerator{}

func (t *testUUIDGenerator) GenerateUUID() (uint64, error) {
	if t.generateUUID == nil {
		panic("generateUUID method is not set")
	}
	return t.generateUUID()
}

type TestTracer struct {
	StartChildSpanFunc func(trace.SpanName, ...otelTrace.SpanStartOption) tracing.TracerSpan
}

var _ environment.Tracer = &TestTracer{}

func (tt *TestTracer) StartChildSpan(
	name trace.SpanName,
	options ...otelTrace.SpanStartOption,
) tracing.TracerSpan {
	// if not set we use noop tracer
	if tt.StartChildSpanFunc == nil {
		return tracing.NewMockTracerSpan()
	}
	return tt.StartChildSpanFunc(name, options...)
}

func (tt *TestTracer) ExpectedSpan(t *testing.T, expected trace.SpanName) {
	tt.StartChildSpanFunc = func(
		sn trace.SpanName,
		sso ...otelTrace.SpanStartOption,
	) tracing.TracerSpan {
		require.Equal(t, expected, sn)
		return tracing.NewMockTracerSpan()
	}
}

type TestMetricsReporter struct {
	SetNumberOfDeployedCOAsFunc func(uint64)
	EVMTransactionExecutedFunc  func(uint64, bool, bool)
	EVMBlockExecutedFunc        func(int, uint64, float64)
}

var _ environment.EVMMetricsReporter = &TestMetricsReporter{}

func (tmr *TestMetricsReporter) SetNumberOfDeployedCOAs(count uint64) {
	// call the method if available otherwise skip
	if tmr.SetNumberOfDeployedCOAsFunc != nil {
		tmr.SetNumberOfDeployedCOAsFunc(count)
	}
}

func (tmr *TestMetricsReporter) EVMTransactionExecuted(gasUsed uint64, isDirectCall bool, failed bool) {
	// call the method if available otherwise skip
	if tmr.EVMTransactionExecutedFunc != nil {
		tmr.EVMTransactionExecutedFunc(gasUsed, isDirectCall, failed)
	}
}

func (tmr *TestMetricsReporter) EVMBlockExecuted(txCount int, totalGasUsed uint64, totalSupplyInFlow float64) {
	// call the method if available otherwise skip
	if tmr.EVMBlockExecutedFunc != nil {
		tmr.EVMBlockExecutedFunc(txCount, totalGasUsed, totalSupplyInFlow)
	}
}

type TestLoggerProvider struct {
	LoggerFunc func() zerolog.Logger
}

func (tlp *TestLoggerProvider) Logger() zerolog.Logger {
	// call the method if not available return noop logger
	if tlp.LoggerFunc != nil {
		return tlp.LoggerFunc()
	}
	return zerolog.Nop()
}
