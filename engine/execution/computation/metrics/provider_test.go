package metrics

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func Test_ProviderGetOnEmpty(t *testing.T) {
	t.Parallel()

	height := uint64(100)
	bufferSize := uint(10)
	log := zerolog.New(zerolog.NewTestWriter(t))

	provider := newProvider(log, bufferSize, height)

	for i := 0; uint(i) < bufferSize; i++ {
		data, err := provider.GetTransactionExecutionMetricsAfter(height - uint64(i))
		require.NoError(t, err)
		require.Len(t, data, 0)
	}
}

func Test_ProviderGetOutOfBounds(t *testing.T) {
	t.Parallel()

	height := uint64(100)
	bufferSize := uint(10)
	log := zerolog.New(zerolog.NewTestWriter(t))

	provider := newProvider(log, bufferSize, height)

	res, err := provider.GetTransactionExecutionMetricsAfter(height + 1)
	require.NoError(t, err)
	require.Len(t, res, 0)
}

func Test_ProviderPushSequential(t *testing.T) {
	t.Parallel()

	height := uint64(100)
	bufferSize := uint(10)
	log := zerolog.New(zerolog.NewTestWriter(t))

	provider := newProvider(log, bufferSize, height)

	for i := 0; uint(i) < bufferSize; i++ {
		data := []TransactionExecutionMetrics{
			{
				// Execution time is our label
				ExecutionTime: time.Duration(i),
			},
		}

		provider.Push(height+uint64(i)+1, data)
	}

	data, err := provider.GetTransactionExecutionMetricsAfter(height)
	require.Nil(t, err)
	for i := 0; uint(i) < bufferSize; i++ {
		require.Equal(t, time.Duration(uint(i)), data[height+uint64(i)+1][0].ExecutionTime)
	}
}

func Test_ProviderPushOutOfSequence(t *testing.T) {
	t.Parallel()

	height := uint64(100)
	bufferSize := uint(10)
	log := zerolog.New(zerolog.NewTestWriter(t))

	provider := newProvider(log, bufferSize, height)

	for i := 0; uint(i) < bufferSize; i++ {
		data := []TransactionExecutionMetrics{
			{
				ExecutionTime: time.Duration(i),
			},
		}

		provider.Push(height+uint64(i)+1, data)
	}

	newHeight := height + uint64(bufferSize)

	// Push out of sequence
	data := []TransactionExecutionMetrics{
		{
			ExecutionTime: time.Duration(newHeight + 2),
		},
	}

	// no-op
	provider.Push(newHeight, data)

	// skip 1
	provider.Push(newHeight+2, data)

	res, err := provider.GetTransactionExecutionMetricsAfter(height)
	require.NoError(t, err)

	require.Len(t, res, int(bufferSize))

	require.Nil(t, res[newHeight+1])
	require.Equal(t, time.Duration(newHeight+2), res[newHeight+2][0].ExecutionTime)
}
