package model_test

import (
	"bytes"
	"testing"

	clone "github.com/huandu/go-clone/generic" //nolint:goimports
	
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// createDummyTimeoutObject constructs a TimeoutObject with dummy data for benchmarking.
func createDummyTimeoutObjects() (*model.TimeoutObject, *model.TimeoutObject) {
	to := helper.TimeoutObjectFixture()
	return to, clone.Clone(to)
}

// comparingUsingFieldsAndHashes compares two TimeoutObjects using field comparisons of primitive types
// and comparison of hashes for the embedded `NewestQC` and `LastViewTC` objects.
func comparingUsingFieldsAndHashes(a, b *model.TimeoutObject) bool {
	if a == b { // shortcut the same pointer is provided
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// both are not nil, so we can compare the fields
	return a.View == b.View &&
		a.NewestQC.ID() == b.NewestQC.ID() &&
		a.LastViewTC.ID() == b.LastViewTC.ID() &&
		a.SignerID == b.SignerID &&
		bytes.Equal(a.SigData, b.SigData)
}

// BenchmarkTimeoutObjectEquals_CompareFieldsAndHashes benchmarks an Equals implementation that compares via fields and hashes.
func BenchmarkTimeoutObjectEquals_CompareFieldsAndHashes(t *testing.B) {
	a, b := createDummyTimeoutObjects()

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_ = comparingUsingFieldsAndHashes(a, b)
	}
}

// comparingOnlyUsingFields compares two TimeoutObjects using only field comparisons.
func comparingOnlyUsingFields(a, b *model.TimeoutObject) bool {
	if a == b { // shortcut the same pointer is provided
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return (a.View == b.View) &&
		(a.SignerID == b.SignerID) &&
		bytes.Equal(a.SigData, b.SigData) &&
		a.NewestQC.Equals(b.NewestQC) &&
		a.LastViewTC.Equals(b.LastViewTC)
}

// BenchmarkTimeoutObjectEquals_CompareFieldsOnly benchmarks an Equals implementation that compares via fields only.
func BenchmarkTimeoutObjectEquals_CompareFieldsOnly(t *testing.B) {
	a, b := createDummyTimeoutObjects()

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_ = comparingOnlyUsingFields(a, b)
	}
}

// BenchmarkTimeoutObjectEquals benchmarks the field comparison using the implementation
func BenchmarkTimeoutObjectEquals(t *testing.B) {
	a, b := createDummyTimeoutObjects()

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_ = a.Equals(b)
	}
}
