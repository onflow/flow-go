package unittest

import (
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

type IntList []uint32

func (l IntList) ID() flow.Identifier {
	return flow.MakeID(l)
}

type EmbeddedStruct struct {
	Identities flow.IdentitySkeletonList
	QC         *flow.QuorumCertificate
}

func (e *EmbeddedStruct) ID() flow.Identifier {
	return flow.MakeID(e)
}

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
		RequireEntityNonMalleable(t, &EmbeddedStruct{
			Identities: IdentityListFixture(2).ToSkeleton(),
			QC:         QuorumCertificateFixture(),
		})
	})
	t.Run("embedded-struct-with-nil-value", func(t *testing.T) {
		RequireEntityNonMalleable(t, &EmbeddedStruct{
			Identities: nil,
			QC:         QuorumCertificateFixture(),
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
