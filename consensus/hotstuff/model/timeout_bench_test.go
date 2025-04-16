package model_test

import (
	"bytes"
	"testing"

	clone "github.com/huandu/go-clone/generic" //nolint:goimports

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// createDummyTimeoutObjects construct two identical TimeoutObject with entirely disjoint memory representation
func createDummyTimeoutObjects() (*model.TimeoutObject, *model.TimeoutObject) {
	to := helper.TimeoutObjectFixture()
	return to, clone.Clone(to)
}

// comparingUsingFieldsAndHashes evaluates equality of two TimeoutObjects using
//   - field comparisons for primitive types
//   - and comparison of hashes for the embedded `NewestQC` and `LastViewTC` objects.
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

// BenchmarkTimeoutObjectEquals_CompareFieldsAndHashes benchmarks `Equals` implementation that is based on
// comparison of fields (for primitive types) and comparison of hashes (for `NewestQC` and `LastViewTC`).
func BenchmarkTimeoutObjectEquals_CompareFieldsAndHashes(t *testing.B) {
	a, b := createDummyTimeoutObjects()

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_ = comparingUsingFieldsAndHashes(a, b)
	}
}

// comparingOnlyUsingFields evaluates equality of two TimeoutObjects using only field comparisons.
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

// BenchmarkTimeoutObjectEquals_CompareFieldsOnly benchmarks `Equals` implementation that is based
// solely on comparison of fields (no serialization and hash computations)
func BenchmarkTimeoutObjectEquals_CompareFieldsOnly(t *testing.B) {
	a, b := createDummyTimeoutObjects()

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_ = comparingOnlyUsingFields(a, b)
	}
}

// BenchmarkTimeoutObjectEquals benchmarks the `Equals` implementation provided by `TimeoutObject`
func BenchmarkTimeoutObjectEquals(t *testing.B) {
	a, b := createDummyTimeoutObjects()

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_ = a.Equals(b)
	}
}
