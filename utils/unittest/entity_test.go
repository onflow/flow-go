package unittest

import (
	clone "github.com/huandu/go-clone/generic"
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
// The default behaviour is if any nil-able fields is nil or size 0, the malleability checker fails.
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

// StructWithNotSettableFlowField will always fail malleability checking because it contains a private (non-settable) field
type StructWithNotSettableFlowField struct {
	field flow.IdentitySkeleton
}

func (e *StructWithNotSettableFlowField) ID() flow.Identifier {
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

// StructWithOptionalField is a struct that has an optional field. This is a rare case but it happens that we need to include
// a field that is optional for backward compatibility reasons. In such cases the ID method might behave differently depending
// on the presence of the optional field. Checker should be able to handle a case where the optional field is nil and when it is not.
// To accomplish this, we are using a special struct tag otherwise the checker would fail to detect the optional field as it requires
// that all fields are non-empty/non-nil.
type StructWithOptionalField struct {
	Identifier    flow.Identifier
	RequiredField uint32
	OptionalField *uint32
}

// ID returns the hash of the entity depending on the presence of the optional field.
func (e *StructWithOptionalField) ID() flow.Identifier {
	if e.OptionalField == nil {
		return flow.MakeID(struct {
			Identifier    flow.Identifier
			RequiredField uint32
		}{
			Identifier:    e.Identifier,
			RequiredField: e.RequiredField,
		})
	} else {
		return flow.MakeID(struct {
			RequiredField uint32
			OptionalField uint32
			Identifier    flow.Identifier
		}{
			Identifier:    e.Identifier,
			OptionalField: *e.OptionalField,
			RequiredField: e.RequiredField,
		})
	}
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
			RequireEntityNonMalleable(t, &StructWithNilFields{
				Identities: nil,
				Index:      map[flow.Identifier]uint32{IdentifierFixture(): 0},
				QC:         QuorumCertificateFixture(),
			})
		})
		t.Run("empty-slice", func(t *testing.T) {
			RequireEntityNonMalleable(t, &StructWithNilFields{
				Identities: make(flow.IdentitySkeletonList, 0),
				Index:      map[flow.Identifier]uint32{IdentifierFixture(): 0},
				QC:         QuorumCertificateFixture(),
			})
		})
		t.Run("nil-map", func(t *testing.T) {
			RequireEntityNonMalleable(t, &StructWithNilFields{
				Identities: IdentityListFixture(5).ToSkeleton(),
				Index:      nil,
				QC:         QuorumCertificateFixture(),
			})
		})
		t.Run("empty-map", func(t *testing.T) {
			RequireEntityNonMalleable(t, &StructWithNilFields{
				Identities: IdentityListFixture(5).ToSkeleton(),
				Index:      map[flow.Identifier]uint32{},
				QC:         QuorumCertificateFixture(),
			})
		})
		t.Run("nil-ptr", func(t *testing.T) {
			RequireEntityNonMalleable(t, &StructWithNilFields{
				Identities: IdentityListFixture(5).ToSkeleton(),
				Index:      map[flow.Identifier]uint32{IdentifierFixture(): 0},
				QC:         nil,
			})
		})
	})
	t.Run("invalid-entity", func(t *testing.T) {
		err := NewMalleabilityChecker().Check(nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "input is not a valid entity")
	})
	t.Run("nil-entity", func(t *testing.T) {
		var e *flow.ExecutionReceipt = nil
		err := NewMalleabilityChecker().Check(e)
		require.Error(t, err)
		require.ErrorContains(t, err, "entity is nil")
	})
	t.Run("unsupported-field", func(t *testing.T) {
		err := NewMalleabilityChecker().Check(&StructWithNotSettableFlowField{
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
	t.Run("struct-with-optional-field", func(t *testing.T) {
		t.Run("without-optional-field", func(t *testing.T) {
			err := NewMalleabilityChecker().Check(&StructWithOptionalField{
				Identifier:    IdentifierFixture(),
				RequiredField: 42,
				OptionalField: nil,
			})
			require.NoError(t, err)
		})
		t.Run("with-optional-field", func(t *testing.T) {
			v := &StructWithOptionalField{
				Identifier:    IdentifierFixture(),
				RequiredField: 42,
				OptionalField: new(uint32),
			}
			*v.OptionalField = 13
			err := NewMalleabilityChecker().Check(v)
			require.NoError(t, err)
		})
	})
}

// EnterViewEvidence is a utility struct for testing the malleability checker when multiple levels of nested structs are involved.
type EnterViewEvidence struct {
	QC *flow.QuorumCertificate
	TC *flow.TimeoutCertificate
}

// StructWithPinning is a struct specifically designed to test the pinning feature of the malleability checker.
type StructWithPinning struct {
	Version  uint32
	Evidence *EnterViewEvidence
}

// ID returns the hash of the entity depending on the value of the Version field.
// Depending on the value of the Version field, the ID method includes or excludes the TC field in the hash calculation and requires it's nil or not in some cases.
// This is a contrived example to demonstrate the pinning feature of the malleability checker.
func (e *StructWithPinning) ID() flow.Identifier {
	if e.Version == 1 {
		if e.Evidence.TC != nil {
			panic("TC should not be set for version 1")
		}
		return flow.MakeID(struct {
			Version uint32
			QcID    flow.Identifier
		}{
			Version: e.Version,
			QcID:    e.Evidence.QC.ID(),
		})
	} else if e.Version == 2 {
		if e.Evidence.QC == nil || e.Evidence.TC == nil {
			panic("QC and TC should be set for version 2")
		}
		return flow.MakeID(struct {
			Version uint32
			QcID    flow.Identifier
			TcID    flow.Identifier
		}{
			Version: e.Version,
			QcID:    e.Evidence.QC.ID(),
			TcID:    e.Evidence.TC.ID(),
		})
	} else {
		panic("unsupported version")
	}
}

// TestMalleabilityChecker_PinField tests the behavior of MalleabilityChecker when pinning is required.
// This structure is implemented in a way that the ID method behaves differently depending on the value of the Version field.
// Depending on the value of the Version field, the ID method includes or excludes the TC field in the hash calculation and requires
// it's nil or not in some cases, this means we need to use pinning otherwise checker will generate random values for the fields.
func TestMalleabilityChecker_PinField(t *testing.T) {
	t.Run("v1", func(t *testing.T) {
		checker := NewMalleabilityChecker(WithPinnedField("Version"), WithPinnedField("Evidence.TC"))
		err := checker.Check(&StructWithPinning{
			Version: 1,
			Evidence: &EnterViewEvidence{
				QC: QuorumCertificateFixture(),
				TC: nil,
			},
		})
		require.NoError(t, err)
	})
	t.Run("v2", func(t *testing.T) {
		checker := NewMalleabilityChecker(WithPinnedField("Version"))
		err := checker.Check(&StructWithPinning{
			Version: 2,
			Evidence: &EnterViewEvidence{
				QC: QuorumCertificateFixture(),
				TC: &flow.TimeoutCertificate{
					View:          0,
					NewestQCViews: nil,
					NewestQC:      nil,
					SignerIndices: nil,
					SigData:       nil,
				},
			},
		})
		require.NoError(t, err)
	})
}

// StructWithComplexType is a struct that contains a slice with complex type.
type StructWithComplexType struct {
	Version   uint32
	Evidences []*EnterViewEvidence
}

func (e *StructWithComplexType) ID() flow.Identifier {
	return flow.MakeID(e)
}

// TestMalleabilityChecker_Generators tests the behavior of MalleabilityChecker when using field and type generators.
// In this test we actually ensure that checker uses the generator and the generated values are set on the entity that is being checked.
func TestMalleabilityChecker_Generators(t *testing.T) {
	t.Run("no-generator", func(t *testing.T) {
		original := &StructWithComplexType{
			Version:   0,
			Evidences: nil,
		}
		cpy := clone.Clone(original)
		RequireEntityNonMalleable(t, cpy)
		require.NotEqual(t, original.Version, cpy.Version)
		require.NotElementsMatch(t, original.Evidences, cpy.Evidences)
	})
	t.Run("field-generator", func(t *testing.T) {
		original := &StructWithComplexType{
			Version:   0,
			Evidences: nil,
		}
		generated := []*EnterViewEvidence{
			{
				QC: QuorumCertificateFixture(),
				TC: nil,
			},
		}
		RequireEntityNonMalleable(t, original, WithFieldGenerator("Evidences", func() []*EnterViewEvidence {
			return generated
		}))
		require.Equal(t, generated, original.Evidences)
	})
	t.Run("type-generator", func(t *testing.T) {
		generated := EnterViewEvidence{
			QC: QuorumCertificateFixture(),
			TC: nil,
		}
		original := &StructWithComplexType{
			Version:   0,
			Evidences: nil,
		}
		RequireEntityNonMalleable(t, original, WithTypeGenerator(func() EnterViewEvidence {
			return generated
		}),
		)
		require.Equal(t, generated, *original.Evidences[0])
	})
}
