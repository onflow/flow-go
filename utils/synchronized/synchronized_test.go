package synchronized

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSynchronizedInt(t *testing.T) {
	synchronizedCounter := New(0)
	synchronizedCounter.WithWriteLock(func(counter *int) {
		*counter += 1
	})

	synchronizedCounter.WithReadLock(func(counter int) {
		require.Equal(t, 1, counter)
	})
}

func TestSynchronizedUserDefinedType(t *testing.T) {
	type Custom struct {
		data int
	}

	expected := Custom{data: 42}
	synchronized := New(expected)

	synchronized.WithReadLock(func(actual Custom) {
		require.Equal(t, expected, actual)
	})
}

func TestSynchronizedSlice(t *testing.T) {
	synchronizedSlice := New([]int{})

	synchronizedSlice.WithWriteLock(func(data *[]int) {
		*data = append(*data, 1, 2, 3)
	})

	synchronizedSlice.WithReadLock(func(data []int) {
		require.Equal(t, []int{1, 2, 3}, data)
	})
}

func TestSynchronizedMap(t *testing.T) {
	synchronizedMap := New(map[string]int{})

	synchronizedMap.WithWriteLock(func(data *map[string]int) {
		(*data)["a"] = 10
		(*data)["b"] = 20
	})

	synchronizedMap.WithReadLock(func(data map[string]int) {
		require.Equal(t, map[string]int{"a": 10, "b": 20}, data)
	})
}

func TestConcurrentAccess(t *testing.T) {
	synchronizedMap := New(map[string]int{})
	done := make(chan bool)

	// Writer
	go func() {
		for i := 0; i < 1000; i++ {
			synchronizedMap.WithWriteLock(func(data *map[string]int) {
				(*data)[string(rune(i%26+'a'))] = i
			})
		}
		done <- true
	}()

	// Reader
	go func() {
		for i := 0; i < 1000; i++ {
			synchronizedMap.WithReadLock(func(data map[string]int) {
				_ = len(data) // read-only operation
			})
		}
		done <- true
	}()

	timeout := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("timeout waiting for goroutines")
		}
	}
}
