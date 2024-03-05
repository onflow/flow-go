package operation

import (
	"testing"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type TestEntity struct {
	Name  string
	Value int
}

func TestEncodeAndCompress(t *testing.T) {
	// Test data
	entity := TestEntity{
		Name:  "TestName",
		Value: 42,
	}

	// Call the function
	compressedVal, err := encodeAndCompress(entity)
	require.NoError(t, err, "encodeAndCompress should not return an error")

	// Ensure the compressed value is not empty
	require.NotEmpty(t, compressedVal, "compressed value should not be empty")

	// Ensure the compressed value is compressed using Snappy
	_, err = snappy.Decode(nil, compressedVal)
	require.NoError(t, err, "should be able to decode the compressed value using Snappy")
}

func TestDecodeCompressed(t *testing.T) {
	// Test data
	entity := TestEntity{
		Name:  "TestName",
		Value: 42,
	}

	// Serialize and compress the test entity
	val, err := encodeAndCompress(entity)
	require.NoError(t, err, "encodeAndCompress should not return an error")

	// Call the function
	var decodedEntity TestEntity
	err = decodeCompressed(val, &decodedEntity)
	require.NoError(t, err, "decodeCompressed should not return an error")

	// Ensure the decoded entity matches the original entity
	require.Equal(t, entity, decodedEntity, "decoded entity should match the original entity")
}

func BenchmarkEncodeAndCompress(b *testing.B) {
	r := unittest.ExecutionResultFixture()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = encodeAndCompress(r)
	}
}

func BenchmarkEncodeWithoutCompress(b *testing.B) {
	r := unittest.ExecutionResultFixture()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = msgpack.Marshal(r)
	}
}

func BenchmarkDecodeUncompressed(b *testing.B) {
	r := unittest.ExecutionResultFixture()

	val, _ := encodeAndCompress(r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result flow.ExecutionResult
		_ = decodeCompressed(val, result)
	}
}

func BenchmarkDecodeWithoutUncompress(b *testing.B) {
	r := unittest.ExecutionResultFixture()
	val, _ := msgpack.Marshal(r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result flow.ExecutionResult
		_ = msgpack.Unmarshal(val, result)
	}
}
