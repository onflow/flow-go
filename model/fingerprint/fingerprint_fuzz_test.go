package fingerprint

import (
	"bytes"
	"testing"
)

// fuzzEntity is a simple exported-enough struct for mutation fuzzing.
type fuzzEntity struct {
	Data []byte
	N    uint64
}

func FuzzFingerprint(f *testing.F) {
	// seeds: fixture-like entities mirroring fingerprint_test.go cases
	f.Add([]byte("abc"), uint64(0))
	f.Add([]byte{0x01, 0xff}, uint64(42))
	f.Add([]byte{}, uint64(0))
	f.Add([]byte("flow-go-boundary-fuzz"), uint64(9999))

	f.Fuzz(func(t *testing.T, data []byte, n uint64) {
		e := &fuzzEntity{Data: data, N: n}

		first := Fingerprint(e)
		second := Fingerprint(e)

		if !bytes.Equal(first, second) {
			t.Fatalf("Fingerprint not deterministic: first=%x second=%x", first, second)
		}
	})
}
