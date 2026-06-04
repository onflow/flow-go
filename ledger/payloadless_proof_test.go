package ledger

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger/common/hash"
)

// TestNewPayloadlessTrieProof verifies the initial state of a freshly constructed proof.
func TestNewPayloadlessTrieProof(t *testing.T) {
	p := NewPayloadlessTrieProof()

	require.Nil(t, p.LeafHash)
	require.NotNil(t, p.Interims)
	require.Equal(t, 0, len(p.Interims))
	require.False(t, p.Inclusion)
	require.Equal(t, PathLen, len(p.Flags))
	require.Equal(t, uint8(0), p.Steps)
}

// TestPayloadlessTrieProofEquals_Identical verifies that two identical proofs compare equal.
func TestPayloadlessTrieProofEquals_Identical(t *testing.T) {
	leafHash := hash.HashLeaf(hash.DummyHash, []byte("v"))

	p1 := NewPayloadlessTrieProof()
	p1.Path = Path(hash.DummyHash)
	p1.LeafHash = &leafHash
	p1.Inclusion = true
	p1.Steps = 5
	p1.Flags[0] = 0x01
	p1.Interims = []hash.Hash{hash.DummyHash}

	p2 := NewPayloadlessTrieProof()
	p2.Path = Path(hash.DummyHash)
	p2LeafHash := leafHash
	p2.LeafHash = &p2LeafHash
	p2.Inclusion = true
	p2.Steps = 5
	p2.Flags[0] = 0x01
	p2.Interims = []hash.Hash{hash.DummyHash}

	require.True(t, p1.Equals(p2))
	require.True(t, p2.Equals(p1))
}

// TestPayloadlessTrieProofEquals_Nil verifies Equals returns false when compared to nil.
func TestPayloadlessTrieProofEquals_Nil(t *testing.T) {
	p := NewPayloadlessTrieProof()
	require.False(t, p.Equals(nil))
}

// TestPayloadlessTrieProofEquals_DifferentPath verifies Equals returns false for different paths.
func TestPayloadlessTrieProofEquals_DifferentPath(t *testing.T) {
	p1 := NewPayloadlessTrieProof()
	p1.Path = Path(hash.DummyHash)

	var otherPath Path
	otherPath[0] = 1
	p2 := NewPayloadlessTrieProof()
	p2.Path = otherPath

	require.False(t, p1.Equals(p2))
}

// TestPayloadlessTrieProofEquals_LeafHash covers all leaf hash nullability/value combinations.
func TestPayloadlessTrieProofEquals_LeafHash(t *testing.T) {
	h1 := hash.HashLeaf(hash.DummyHash, []byte("a"))
	h2 := hash.HashLeaf(hash.DummyHash, []byte("b"))

	t.Run("both nil", func(t *testing.T) {
		p1 := NewPayloadlessTrieProof()
		p2 := NewPayloadlessTrieProof()
		require.True(t, p1.Equals(p2))
	})

	t.Run("one nil one non-nil", func(t *testing.T) {
		p1 := NewPayloadlessTrieProof()
		p2 := NewPayloadlessTrieProof()
		p2.LeafHash = &h1
		require.False(t, p1.Equals(p2))
		require.False(t, p2.Equals(p1))
	})

	t.Run("both non-nil equal", func(t *testing.T) {
		p1 := NewPayloadlessTrieProof()
		p2 := NewPayloadlessTrieProof()
		h1Copy := h1
		p1.LeafHash = &h1
		p2.LeafHash = &h1Copy
		require.True(t, p1.Equals(p2))
	})

	t.Run("both non-nil different", func(t *testing.T) {
		p1 := NewPayloadlessTrieProof()
		p2 := NewPayloadlessTrieProof()
		p1.LeafHash = &h1
		p2.LeafHash = &h2
		require.False(t, p1.Equals(p2))
	})
}

// TestPayloadlessTrieProofEquals_Inclusion verifies Equals respects the Inclusion flag.
func TestPayloadlessTrieProofEquals_Inclusion(t *testing.T) {
	p1 := NewPayloadlessTrieProof()
	p1.Inclusion = true
	p2 := NewPayloadlessTrieProof()
	p2.Inclusion = false
	require.False(t, p1.Equals(p2))
}

// TestPayloadlessTrieProofEquals_Steps verifies Equals respects Steps.
func TestPayloadlessTrieProofEquals_Steps(t *testing.T) {
	p1 := NewPayloadlessTrieProof()
	p1.Steps = 1
	p2 := NewPayloadlessTrieProof()
	p2.Steps = 2
	require.False(t, p1.Equals(p2))
}

// TestPayloadlessTrieProofEquals_Flags verifies Equals respects Flags.
func TestPayloadlessTrieProofEquals_Flags(t *testing.T) {
	p1 := NewPayloadlessTrieProof()
	p1.Flags[0] = 0x01
	p2 := NewPayloadlessTrieProof()
	p2.Flags[0] = 0x02
	require.False(t, p1.Equals(p2))
}

// TestPayloadlessTrieProofEquals_Interims verifies Equals respects Interims (length and content).
func TestPayloadlessTrieProofEquals_Interims(t *testing.T) {
	t.Run("different length", func(t *testing.T) {
		p1 := NewPayloadlessTrieProof()
		p1.Interims = []hash.Hash{hash.DummyHash}
		p2 := NewPayloadlessTrieProof()
		p2.Interims = []hash.Hash{hash.DummyHash, hash.DummyHash}
		require.False(t, p1.Equals(p2))
	})

	t.Run("different content", func(t *testing.T) {
		var h1, h2 hash.Hash
		h1[0] = 1
		h2[0] = 2
		p1 := NewPayloadlessTrieProof()
		p1.Interims = []hash.Hash{h1}
		p2 := NewPayloadlessTrieProof()
		p2.Interims = []hash.Hash{h2}
		require.False(t, p1.Equals(p2))
	})
}

// TestPayloadlessTrieProofString_DoesNotPanic exercises the String method on
// representative shapes (inclusion vs non-inclusion, with and without LeafHash).
func TestPayloadlessTrieProofString_DoesNotPanic(t *testing.T) {
	p := NewPayloadlessTrieProof()
	require.NotEmpty(t, p.String()) // empty/non-inclusion proof

	p.Inclusion = true
	leafHash := hash.HashLeaf(hash.DummyHash, []byte("x"))
	p.LeafHash = &leafHash
	p.Interims = []hash.Hash{hash.DummyHash}
	p.Steps = 1
	p.Flags[0] = 0x80
	require.NotEmpty(t, p.String())
}

// TestNewPayloadlessTrieBatchProof verifies the empty batch proof.
func TestNewPayloadlessTrieBatchProof(t *testing.T) {
	bp := NewPayloadlessTrieBatchProof()
	require.NotNil(t, bp)
	require.NotNil(t, bp.Proofs)
	require.Equal(t, 0, bp.Size())
}

// TestNewPayloadlessTrieBatchProofWithEmptyProofs verifies the pre-filled batch proof.
func TestNewPayloadlessTrieBatchProofWithEmptyProofs(t *testing.T) {
	bp := NewPayloadlessTrieBatchProofWithEmptyProofs(3)
	require.Equal(t, 3, bp.Size())
	for _, p := range bp.Proofs {
		require.NotNil(t, p)
		require.False(t, p.Inclusion)
		require.Nil(t, p.LeafHash)
		require.Equal(t, PathLen, len(p.Flags))
	}
}

// TestPayloadlessTrieBatchProof_PathsAndLeafHashes verifies that Paths and LeafHashes
// return slices aligned with the underlying Proofs.
func TestPayloadlessTrieBatchProof_PathsAndLeafHashes(t *testing.T) {
	var p1Path, p2Path Path
	p1Path[0] = 1
	p2Path[0] = 2
	h1 := hash.HashLeaf(hash.Hash(p1Path), []byte("a"))

	bp := NewPayloadlessTrieBatchProofWithEmptyProofs(2)
	bp.Proofs[0].Path = p1Path
	bp.Proofs[0].LeafHash = &h1
	bp.Proofs[0].Inclusion = true
	bp.Proofs[1].Path = p2Path
	// Proofs[1].LeafHash stays nil (non-inclusion)

	require.Equal(t, []Path{p1Path, p2Path}, bp.Paths())

	leafHashes := bp.LeafHashes()
	require.Equal(t, 2, len(leafHashes))
	require.NotNil(t, leafHashes[0])
	require.Equal(t, h1, *leafHashes[0])
	require.Nil(t, leafHashes[1])
}

// TestPayloadlessTrieBatchProof_AppendProof verifies AppendProof grows the batch.
func TestPayloadlessTrieBatchProof_AppendProof(t *testing.T) {
	bp := NewPayloadlessTrieBatchProof()
	require.Equal(t, 0, bp.Size())

	bp.AppendProof(NewPayloadlessTrieProof())
	require.Equal(t, 1, bp.Size())

	bp.AppendProof(NewPayloadlessTrieProof())
	require.Equal(t, 2, bp.Size())
}

// TestPayloadlessTrieBatchProof_MergeInto verifies MergeInto appends proofs to a destination.
func TestPayloadlessTrieBatchProof_MergeInto(t *testing.T) {
	src := NewPayloadlessTrieBatchProofWithEmptyProofs(2)
	dest := NewPayloadlessTrieBatchProofWithEmptyProofs(3)

	src.MergeInto(dest)
	require.Equal(t, 5, dest.Size())
	require.Equal(t, 2, src.Size()) // source unchanged in size
}

// TestPayloadlessTrieBatchProof_String_DoesNotPanic exercises the String method.
func TestPayloadlessTrieBatchProof_String_DoesNotPanic(t *testing.T) {
	bp := NewPayloadlessTrieBatchProofWithEmptyProofs(2)
	require.NotEmpty(t, bp.String())
}

// TestPayloadlessTrieBatchProofEquals verifies the equality semantics of a batch proof.
func TestPayloadlessTrieBatchProofEquals(t *testing.T) {
	build := func() *PayloadlessTrieBatchProof {
		var p1Path Path
		p1Path[0] = 1
		h1 := hash.HashLeaf(hash.Hash(p1Path), []byte("a"))

		bp := NewPayloadlessTrieBatchProofWithEmptyProofs(1)
		bp.Proofs[0].Path = p1Path
		bp.Proofs[0].LeafHash = &h1
		bp.Proofs[0].Inclusion = true
		return bp
	}

	t.Run("identical", func(t *testing.T) {
		require.True(t, build().Equals(build()))
	})

	t.Run("nil", func(t *testing.T) {
		require.False(t, build().Equals(nil))
	})

	t.Run("different size", func(t *testing.T) {
		bp1 := build()
		bp2 := build()
		bp2.AppendProof(NewPayloadlessTrieProof())
		require.False(t, bp1.Equals(bp2))
	})

	t.Run("different proof content", func(t *testing.T) {
		bp1 := build()
		bp2 := build()
		bp2.Proofs[0].Inclusion = false
		require.False(t, bp1.Equals(bp2))
	})
}
