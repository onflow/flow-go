package unittest

import (
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/model/flow"
)

type IntList []uint32

func (l IntList) ID() flow.Identifier {
	return flow.MakeID(l)
}

// StructWithNilFields allows testing all cases of nil-able fields: struct pointer, slice, map.
// The default behaviour is if any nilable fields is nil or size 0, the malleability checker fails.
type StructWithNilFields struct {
	Identities flow.IdentitySkeletonList
	Index      map[flow.Identifier]uint32
	QC         *flow.QuorumCertificate
}

func (e *StructWithNilFields) ID() flow.Identifier {
	type pair struct {
		ID    flow.Identifier
		Index uint32
	}
	var pairs []pair
	for id, index := range e.Index {
		pairs = append(pairs, pair{id, index})
	}
	slices.SortFunc(pairs, func(a, b pair) int {
		return flow.IdentifierCanonical(a.ID, b.ID)
	})
	return flow.MakeID(struct {
		Identities flow.IdentitySkeletonList
		Index      []pair
		QcID       flow.Identifier
	}{
		Identities: e.Identities,
		Index:      pairs,
		QcID:       e.QC.ID(),
	})
}

// StructWithUnsupportedFlowField will always fail malleability checking because it contains a private (non-settable) field
type StructWithUnsupportedFlowField struct {
	field flow.IdentitySkeleton
}

func (e *StructWithUnsupportedFlowField) ID() flow.Identifier {
	return flow.MakeID(e)
}

// MalleableEntityStruct is a struct that is malleable because its ID method does not cover all of its fields.
type MalleableEntityStruct struct {
	Identities flow.IdentitySkeletonList
	QC         *flow.QuorumCertificate
	Signature  crypto.Signature
}

// ID returns the hash of the entity in a way that does not cover all of its fields.
func (e *MalleableEntityStruct) ID() flow.Identifier {
	return flow.MakeID(struct {
		Identities flow.IdentitySkeletonList
		QcID       flow.Identifier
	}{
		Identities: e.Identities,
		QcID:       e.QC.ID(),
	})
}

// TestRequireEntityNonMalleable tests the behavior of MalleabilityChecker with different types of entities ensuring
// it correctly handles the supported types and returns an error when the entity is malleable, or it cannot perform the check.
func TestRequireEntityNonMalleable(t *testing.T) {
	t.Run("type alias", func(t *testing.T) {
		list := &IntList{1, 2, 3}
		RequireEntityNonMalleable(t, list)
	})
	t.Run("embedded-struct", func(t *testing.T) {
		RequireEntityNonMalleable(t, &StructWithNilFields{
			Identities: IdentityListFixture(2).ToSkeleton(),
			Index:      map[flow.Identifier]uint32{IdentifierFixture(): 0},
			QC:         QuorumCertificateFixture(),
		})
	})
	t.Run("embedded-struct-with-nil-value", func(t *testing.T) {
		t.Run("nil-slice", func(t *testing.T) {
			err := NewMalleabilityChecker().Check(&StructWithNilFields{
				Identities: nil,
				Index:      map[flow.Identifier]uint32{IdentifierFixture(): 0},
				QC:         QuorumCertificateFixture(),
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "invalid entity, map/slice is empty")
		})
		t.Run("empty-slice", func(t *testing.T) {
			err := NewMalleabilityChecker().Check(&StructWithNilFields{
				Identities: make(flow.IdentitySkeletonList, 0),
				Index:      map[flow.Identifier]uint32{IdentifierFixture(): 0},
				QC:         QuorumCertificateFixture(),
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "invalid entity, map/slice is empty")
		})
		t.Run("nil-map", func(t *testing.T) {
			err := NewMalleabilityChecker().Check(&StructWithNilFields{
				Identities: IdentityListFixture(5).ToSkeleton(),
				Index:      nil,
				QC:         QuorumCertificateFixture(),
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "invalid entity, map/slice is empty")
		})
		t.Run("empty-map", func(t *testing.T) {
			err := NewMalleabilityChecker().Check(&StructWithNilFields{
				Identities: IdentityListFixture(5).ToSkeleton(),
				Index:      map[flow.Identifier]uint32{},
				QC:         QuorumCertificateFixture(),
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "invalid entity, map/slice is empty")
		})
		t.Run("nil-ptr", func(t *testing.T) {
			err := NewMalleabilityChecker().Check(&StructWithNilFields{
				Identities: IdentityListFixture(5).ToSkeleton(),
				Index:      map[flow.Identifier]uint32{IdentifierFixture(): 0},
				QC:         nil,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "invalid entity, field is nil")
		})
	})
	t.Run("invalid-entity", func(t *testing.T) {
		err := NewMalleabilityChecker().Check(nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "tested entity is not valid")
	})
	t.Run("nil-entity", func(t *testing.T) {
		var e *flow.ExecutionReceipt = nil
		err := NewMalleabilityChecker().Check(e)
		require.Error(t, err)
		require.ErrorContains(t, err, "entity is nil")
	})
	t.Run("unsupported-field", func(t *testing.T) {
		err := NewMalleabilityChecker().Check(&StructWithUnsupportedFlowField{
			field: IdentityFixture().IdentitySkeleton,
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "not settable")
	})
	t.Run("malleable-entity", func(t *testing.T) {
		err := NewMalleabilityChecker().Check(&MalleableEntityStruct{
			Identities: IdentityListFixture(2).ToSkeleton(),
			QC:         QuorumCertificateFixture(),
			Signature:  SignatureFixture(),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "Signature is malleable")
	})
}
