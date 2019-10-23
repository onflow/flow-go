package gossip

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//testing the receivedCache of the memoryCache
//The first two test cases are inserted into the cache and the rest two are not.
//The receive function is then evaluated against the test cases
func TestReceive(t *testing.T) {
	mhc := newMemoryHashCache()

	//The sole keys that should exist in the cache
	initKeys := []string{
		"exists",
		"found",
	}

	for _, el := range initKeys {
		mhc.receive(el)
	}

	tt := []struct {
		item  string
		found bool
	}{
		{ //an existing key in the cache
			item:  "exists",
			found: true,
		},
		{ //an existing key in the cache
			item:  "found",
			found: true,
		},
		{ //a non-existing key in the cache
			item:  "doesntexist",
			found: false,
		},
		{ //a non-existing key in the cache
			item:  "notfound",
			found: false,
		},
	}

	for _, tc := range tt {
		found := mhc.isReceived(tc.item)
		if found != tc.found {
			t.Errorf("wrong output in receiveTest, Got: %v, Expected: %v", found, tc.found)
		}
	}

}

//testing the confirmCache of the memoryCache
//The first two test cases are inserted into the cache and the rest two are not.
//The confirm function is then evaluated against the test cases
func TestConfirm(t *testing.T) {
	mhc := newMemoryHashCache()

	initKeys := []string{
		"exists",
		"found",
	}

	for _, el := range initKeys {
		mhc.confirm(el)
	}

	tt := []struct {
		item  string
		found bool
	}{
		{ //an existing key in the cache
			item:  "exists",
			found: true,
		},
		{ //an existing key in the cache
			item:  "found",
			found: true,
		},
		{ //a non-existing key in the cache
			item:  "doesntexist",
			found: false,
		},
		{ //a non-existing key in the cache
			item:  "notfound",
			found: false,
		},
	}

	for _, tc := range tt {
		found := mhc.isConfirmed(tc.item)
		if found != tc.found {
			t.Errorf("wrong output in confirmTest, Got: %v, Expected: %v", found, tc.found)
		}
	}
}

//testing the confirmCache of the memoryCache
//The first two test cases are inserted into the memory set and the rest two are not.
//The memorySet is then evaluated against the test cases
func TestMemorySet(t *testing.T) {
	ms := newMemorySet()

	initKeys := []string{
		"exists",
		"found",
	}

	for _, el := range initKeys {
		ms.put(el)
	}

	tt := []struct {
		item  string
		found bool
	}{
		{ //an existing key in the cache
			item:  "exists",
			found: true,
		},
		{ //an existing key in the cache
			item:  "found",
			found: true,
		},
		{ //a non-existing key in the cache
			item:  "doesntexist",
			found: false,
		},
		{ //a non-existing key in the cache
			item:  "notfound",
			found: false,
		},
	}

	for _, tc := range tt {
		found := ms.contains(tc.item)
		assert.Equal(t, found, tc.found)
		if found != tc.found {
			t.Errorf("wrong output in confirmTest, Got: %v, Expected: %v", found, tc.found)
		}
	}
}
