package unittest

import (
	"testing"

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
	Field chan struct{}
}

func (e *StructWithUnsupportedFlowField) ID() flow.Identifier {
	return flow.MakeID(e)
}

// TestRequireEntityNonMalleable tests the RequireEntityNonMalleable function with different types of entities ensuring
// it correctly handles the supported types and panics when the entity is not supported.
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
	t.Run("nil-entity", func(t *testing.T) {
		require.Panics(t, func() {
			RequireEntityNonMalleable(t, nil)
		})
	})
	t.Run("unsupported-field", func(t *testing.T) {
		require.Panics(t, func() {
			RequireEntityNonMalleable(t, &StructWithUnsupportedFlowField{
				Field: make(chan struct{}),
			})
		})
	})
}
