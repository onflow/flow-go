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
	"github.com/onflow/flow-go/utils/unittest"
)

var TestFlowEVMRootAddress = flow.Address{1, 2, 3, 4}
var TestComputationLimit = uint64(100_000_000)

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
			e, err := flow.NewEvent(
				flow.UntrustedEvent{
					Type:             flow.EventType(event.EventType.ID()),
					TransactionID:    unittest.IdentifierFixture(),
					TransactionIndex: 0,
					EventIndex:       0,
					Payload:          payload,
				},
			)
			if err != nil {
				return fmt.Errorf("could not construct event: %w", err)
			}

			events = append(events, *e)
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
	compUsed := uint64(0)
	return &testMeter{
		meterComputation: func(usage common.ComputationUsage) error {
			compUsed += usage.Intensity
			if compUsed > TestComputationLimit {
				return fmt.Errorf("computation limit has hit %d", TestComputationLimit)
			}
			return nil
		},
		hasComputationCapacity: func(usage common.ComputationUsage) bool {
			return compUsed+usage.Intensity < TestComputationLimit
		},
		computationUsed: func() (uint64, error) {
			return compUsed, nil
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
	getValueFunc := vs.GetValueFunc
	if getValueFunc == nil {
		panic("method not set")
	}
	return getValueFunc(owner, key)
}

func (vs *TestValueStore) SetValue(owner, key, value []byte) error {
	setValueFunc := vs.SetValueFunc
	if setValueFunc == nil {
		panic("method not set")
	}
	return setValueFunc(owner, key, value)
}

func (vs *TestValueStore) ValueExists(owner, key []byte) (bool, error) {
	valueExistsFunc := vs.ValueExistsFunc
	if valueExistsFunc == nil {
		panic("method not set")
	}
	return valueExistsFunc(owner, key)
}

func (vs *TestValueStore) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	allocateSlabIndexFunc := vs.AllocateSlabIndexFunc
	if allocateSlabIndexFunc == nil {
		panic("method not set")
	}
	return allocateSlabIndexFunc(owner)
}

func (vs *TestValueStore) TotalBytesRead() int {
	totalBytesReadFunc := vs.TotalBytesReadFunc
	if totalBytesReadFunc == nil {
		panic("method not set")
	}
	return totalBytesReadFunc()
}

func (vs *TestValueStore) TotalBytesWritten() int {
	totalBytesWrittenFunc := vs.TotalBytesWrittenFunc
	if totalBytesWrittenFunc == nil {
		panic("method not set")
	}
	return totalBytesWrittenFunc()
}

func (vs *TestValueStore) TotalStorageSize() int {
	totalStorageSizeFunc := vs.TotalStorageSizeFunc
	if totalStorageSizeFunc == nil {
		panic("method not set")
	}
	return totalStorageSizeFunc()
}

func (vs *TestValueStore) TotalStorageItems() int {
	totalStorageItemsFunc := vs.TotalStorageItemsFunc
	if totalStorageItemsFunc == nil {
		panic("method not set")
	}
	return totalStorageItemsFunc()
}

func (vs *TestValueStore) ResetStats() {
	resetStatsFunc := vs.ResetStatsFunc
	if resetStatsFunc == nil {
		panic("method not set")
	}
	resetStatsFunc()
}

func (vs *TestValueStore) Clone() *TestValueStore {
	cloneFunc := vs.CloneFunc
	if cloneFunc == nil {
		panic("method not set")
	}
	return cloneFunc()
}

func (vs *TestValueStore) Dump() (map[string][]byte, map[string]uint64) {
	dumpFunc := vs.DumpFunc
	if dumpFunc == nil {
		panic("method not set")
	}
	return dumpFunc()
}

type testMeter struct {
	meterComputation       func(usage common.ComputationUsage) error
	hasComputationCapacity func(common.ComputationUsage) bool
	computationUsed        func() (uint64, error)
	computationIntensities func() meter.MeteredComputationIntensities

	meterMemory func(usage common.MemoryUsage) error
	memoryUsed  func() (uint64, error)

	meterEmittedEvent      func(byteSize uint64) error
	totalEmittedEventBytes func() uint64

	interactionUsed func() (uint64, error)

	disabled bool
}

var _ environment.Meter = &testMeter{}

func (m *testMeter) MeterComputation(usage common.ComputationUsage) error {
	if m.disabled {
		return nil
	}
	meterComputation := m.meterComputation
	if meterComputation == nil {
		panic("method not set")
	}
	return meterComputation(usage)
}

func (m *testMeter) ComputationAvailable(usage common.ComputationUsage) bool {
	hasComputationCapacity := m.hasComputationCapacity
	if hasComputationCapacity == nil {
		panic("method not set")
	}
	return hasComputationCapacity(usage)
}

func (m *testMeter) ComputationIntensities() meter.MeteredComputationIntensities {
	computationIntensities := m.computationIntensities
	if computationIntensities == nil {
		panic("method not set")
	}
	return computationIntensities()
}

func (m *testMeter) ComputationUsed() (uint64, error) {
	computationUsed := m.computationUsed
	if computationUsed == nil {
		panic("method not set")
	}
	return computationUsed()
}

func (m *testMeter) RunWithMeteringDisabled(f func()) {
	disabled := m.disabled
	m.disabled = true
	f()
	m.disabled = disabled
}

func (m *testMeter) MeterMemory(usage common.MemoryUsage) error {
	if m.disabled {
		return nil
	}
	meterMemory := m.meterMemory
	if meterMemory == nil {
		panic("method not set")
	}
	return meterMemory(usage)
}

func (m *testMeter) MemoryUsed() (uint64, error) {
	memoryUsed := m.memoryUsed
	if memoryUsed == nil {
		panic("method not set")
	}
	return memoryUsed()
}

func (m *testMeter) InteractionUsed() (uint64, error) {
	interactionUsed := m.interactionUsed
	if interactionUsed == nil {
		panic("method not set")
	}
	return interactionUsed()
}

func (m *testMeter) MeterEmittedEvent(byteSize uint64) error {
	if m.disabled {
		return nil
	}
	meterEmittedEvent := m.meterEmittedEvent
	if meterEmittedEvent == nil {
		panic("method not set")
	}
	return meterEmittedEvent(byteSize)
}

func (m *testMeter) TotalEmittedEventBytes() uint64 {
	totalEmittedEventBytes := m.totalEmittedEventBytes
	if totalEmittedEventBytes == nil {
		panic("method not set")
	}
	return totalEmittedEventBytes()
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
	emitEvent := vs.emitEvent
	if emitEvent == nil {
		panic("method not set")
	}
	return emitEvent(event)
}

func (vs *testEventEmitter) Events() flow.EventsList {
	events := vs.events
	if events == nil {
		panic("method not set")
	}
	return events()
}

func (vs *testEventEmitter) ServiceEvents() flow.EventsList {
	serviceEvents := vs.serviceEvents
	if serviceEvents == nil {
		panic("method not set")
	}
	return serviceEvents()
}

func (vs *testEventEmitter) ConvertedServiceEvents() flow.ServiceEventList {
	convertedServiceEvents := vs.convertedServiceEvents
	if convertedServiceEvents == nil {
		panic("method not set")
	}
	return convertedServiceEvents()
}

func (vs *testEventEmitter) Reset() {
	reset := vs.reset
	if reset == nil {
		panic("method not set")
	}
	reset()
}

type TestBlockInfo struct {
	GetCurrentBlockHeightFunc func() (uint64, error)
	GetBlockAtHeightFunc      func(height uint64) (runtime.Block, bool, error)
}

var _ environment.BlockInfo = &TestBlockInfo{}

// GetCurrentBlockHeight returns the current block height.
func (tb *TestBlockInfo) GetCurrentBlockHeight() (uint64, error) {
	getCurrentBlockHeightFunc := tb.GetCurrentBlockHeightFunc
	if getCurrentBlockHeightFunc == nil {
		panic("GetCurrentBlockHeight method is not set")
	}
	return getCurrentBlockHeightFunc()
}

// GetBlockAtHeight returns the block at the given height.
func (tb *TestBlockInfo) GetBlockAtHeight(height uint64) (runtime.Block, bool, error) {
	getBlockAtHeightFunc := tb.GetBlockAtHeightFunc
	if getBlockAtHeightFunc == nil {
		panic("GetBlockAtHeight method is not set")
	}
	return getBlockAtHeightFunc(height)
}

type TestRandomGenerator struct {
	ReadRandomFunc func([]byte) error
}

var _ environment.RandomGenerator = &TestRandomGenerator{}

func (t *TestRandomGenerator) ReadRandom(buffer []byte) error {
	readRandomFunc := t.ReadRandomFunc
	if readRandomFunc == nil {
		panic("ReadRandomFunc method is not set")
	}
	return readRandomFunc(buffer)
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
	invokeFunc := t.InvokeFunc
	if invokeFunc == nil {
		panic("InvokeFunc method is not set")
	}
	return invokeFunc(spec, arguments)
}

type testUUIDGenerator struct {
	generateUUID func() (uint64, error)
}

var _ environment.UUIDGenerator = &testUUIDGenerator{}

func (t *testUUIDGenerator) GenerateUUID() (uint64, error) {
	generateUUID := t.generateUUID
	if generateUUID == nil {
		panic("generateUUID method is not set")
	}
	return generateUUID()
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
	startChildSpanFunc := tt.StartChildSpanFunc
	if startChildSpanFunc == nil {
		return tracing.NewMockTracerSpan()
	}
	return startChildSpanFunc(name, options...)
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
	setNumberOfDeployedCOAsFunc := tmr.SetNumberOfDeployedCOAsFunc
	if setNumberOfDeployedCOAsFunc != nil {
		setNumberOfDeployedCOAsFunc(count)
	}
}

func (tmr *TestMetricsReporter) EVMTransactionExecuted(gasUsed uint64, isDirectCall bool, failed bool) {
	// call the method if available otherwise skip
	evmTransactionExecutedFunc := tmr.EVMTransactionExecutedFunc
	if evmTransactionExecutedFunc != nil {
		evmTransactionExecutedFunc(gasUsed, isDirectCall, failed)
	}
}

func (tmr *TestMetricsReporter) EVMBlockExecuted(txCount int, totalGasUsed uint64, totalSupplyInFlow float64) {
	// call the method if available otherwise skip
	evmBlockExecutedFunc := tmr.EVMBlockExecutedFunc
	if evmBlockExecutedFunc != nil {
		evmBlockExecutedFunc(txCount, totalGasUsed, totalSupplyInFlow)
	}
}

type TestLoggerProvider struct {
	LoggerFunc func() zerolog.Logger
}

func (tlp *TestLoggerProvider) Logger() zerolog.Logger {
	// call the method if not available return noop logger
	loggerFunc := tlp.LoggerFunc
	if loggerFunc != nil {
		return loggerFunc()
	}
	return zerolog.Nop()
}
