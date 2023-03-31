package cached

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func randIntPtr() *int {
	val := rand.Int()
	return &val
}

func TestCachedOnce_SetGet(t *testing.T) {
	cache := NewWriteOnce[int]()

	val, ok := cache.Get()
	require.Nil(t, val)
	require.False(t, ok)

	initial := randIntPtr()
	ok = cache.Set(initial)
	require.True(t, ok)

	val, ok = cache.Get()
	require.Equal(t, initial, val)
	require.True(t, ok)

	ok = cache.Set(randIntPtr())
	require.False(t, ok)

	val, ok = cache.Get()
	require.Equal(t, initial, val)
	require.True(t, ok)
}

func TestCachedOnce_MultiThreaded(t *testing.T) {
	nReaders := 20
	nSetters := 20

	cache := NewWriteOnce[int]()

	// signal to stop read routines
	stopReaders := make(chan struct{})
	// contains the set value, for every setter routine which successfully set
	// a value (should only contain one item)
	setVals := make(chan int, nSetters)
	// contains the first read value, for every getter routine (should all match
	// single item in setVals)
	readVals := make(chan int, nReaders)

	// start up some reader goroutines
	// each reader will:
	//  - attempt to read the cached value in a loop, until stopped
	//  - store the first cached value it reads
	//  - validate that the cached value does not change after first read
	readersStopped := new(sync.WaitGroup)
	readersStopped.Add(nReaders)
	for i := 0; i < nReaders; i++ {
		go func() {
			defer readersStopped.Done()
			readOK := false  // read a value successfully at least once
			var readVal *int // the value of the first read
			for {
				select {
				case <-stopReaders:
					return
				default:
				}
				val, ok := cache.Get()
				if !ok {
					require.Nil(t, val)
					require.False(t, readOK, "should never get nil after getting a cached value")
					continue
				}
				// once: signal and store read value on the first successful read
				if !readOK {
					readOK = true
					readVal = val
					readVals <- *readVal
				}
				// every time: subsequent read value must match first read
				require.Equal(t, readVal, val)
			}
		}()
	}

	// starts some setter goroutines
	// each setter will:
	//  - attempt to set a random value in the cache
	//  - if it successfully sets a value, signal that value over setVals
	settersStopped := new(sync.WaitGroup)
	settersStopped.Add(nSetters)
	for i := 0; i < nSetters; i++ {
		go func() {
			settingVal := randIntPtr()
			defer settersStopped.Done()
			ok := cache.Set(settingVal)
			if ok {
				setVals <- *settingVal
			}
		}()
	}

	fmt.Println("waiting for setters to stopReaders")
	settersStopped.Wait()

	close(setVals)
	cachedVal := <-setVals
	unittest.AssertClosedChannelIsEmpty[int](t, setVals)

	fmt.Println("waiting for readers to get val")
	for i := 0; i < nReaders; i++ {
		assert.Equal(t, cachedVal, <-readVals)
	}

	fmt.Println("stopping readers")
	close(stopReaders)
	fmt.Println("waiting for reads to stopReaders")
	readersStopped.Wait()
}
