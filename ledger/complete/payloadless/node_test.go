package payloadless

// White-box tests for the payloadless Node constructors. They live in `package payloadless`
// (not `payloadless_test`) so they can exercise the un-exported constructors `newDefaultLeaf`
// and `newLeafWithHash` and inspect internal fields (`leafHash`, `path`, `height`, `hashValue`,
// `lChild`, `rChild`) directly.
//
// Hash-correctness is verified three ways:
//   - Unrolled independent composition for small heights (0,1,2): we spell out the expected hash by
//     hand — nesting `HashInterNode` level by level with empty siblings — rather than reusing the
//     production loop in `ledger.ComputeCompactValueFromLeafHash`. Using left-, right-, and
//     mixed-branching paths, this is an independent cross-check that the constructor computes the
//     same hash.
//   - Cross-construction equivalences: relevelling a leaf must equal freshly constructing it at the
//     target height; a compactified leaf must equal its fully-expanded `NewInterimNode` reference.
//   - Python reference-implementation anchors: node hashes for fixed (path, value, height) inputs,
//     independently computed by the standalone Python Merkle reference implementation (the source of
//     truth) and hard-coded here as an oracle. A compactified leaf at height h holding one register
//     has the same hash as the root of a height-h subtree containing only it, so the reference
//     computes these node hashes from first principles (its node-level scenario section prints them).
//     Reference: github.com/onflow/flow-internal → reference_implementations/merkle_tree.py; run it to
//     regenerate. This is the same convention the sibling mtrie trie tests use (mtrie/trie/trie_test.go,
//     e.g. Test_TrieWithLeftRegister). A mismatch means this Go implementation and the reference
//     disagree (e.g. a change in the hashing primitives or default-hash table).

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

// Shared path fixtures covering the three branching regimes a compactified leaf's fold can take. A
// leaf at height h folds up h levels; at each level the path bit selects which side the (default)
// empty sibling sits on. Testing only uniform paths (all-left or all-right) would miss a wrong-bit-
// index bug, since both boundaries are symmetric under such a bug; a mixed path is the discriminator.
//   - `pathLeft`  = all bits 0  → left branch at every height  (empty sibling always on the RIGHT);
//   - `pathRight` = all bits 1  → right branch at every height (empty sibling always on the LEFT);
//   - `pathMixed` = 0x…0135     → interleaved: right,left,right,left,right,right,left,left,right over
//     heights 1..9 (bit(255)=1, bit(254)=0, …).
var (
	pathLeft = testutils.PathByUint16LeftPadded(0) // 32 bytes 0x00
	// pathRight is the all-ones path (32 bytes 0xFF): branches right at every height.
	pathRight = ledger.Path{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}
	pathMixed = testutils.PathByUint16LeftPadded(0x135) // ...0000_0001_0011_0101
	value     = []byte{0x9a, 0xbc, 0xde}
)

// pathFixture names one of the three branching-regime path fixtures above.
type pathFixture struct {
	name string
	path ledger.Path
}

// branchRegimePaths is the left-only, right-only, and mixed path fixtures, for sweeping any
// branch-sensitive scenario across all three regimes.
var branchRegimePaths = []pathFixture{
	{"pathLeft", pathLeft},
	{"pathRight", pathRight},
	{"pathMixed", pathMixed},
}

// ---------------------------------------------------------------------------------------------
// NewLeaf
// ---------------------------------------------------------------------------------------------

// Test_NewLeaf_Allocated verifies NewLeaf for a non-empty value (an allocated register). At height 0
// the node hash equals the height-0 leaf hash; at height > 0 it equals the fully-expanded reference.
func Test_NewLeaf_Allocated(t *testing.T) {
	leafHashL := hash.HashLeaf(hash.Hash(pathLeft), value)  // height-0 leaf hash for pathLeft
	leafHashR := hash.HashLeaf(hash.Hash(pathRight), value) // ... for pathRight
	leafHashM := hash.HashLeaf(hash.Hash(pathMixed), value) // ... for pathMixed

	t.Run("height 0 is an uncompactified leaf", func(t *testing.T) {
		n := NewLeaf(pathLeft, value, 0)
		require.True(t, n.IsLeaf())
		require.Nil(t, n.lChild)
		require.Nil(t, n.rChild)
		require.Equal(t, 0, n.height)
		require.Equal(t, pathLeft, *n.Path())
		require.NotNil(t, n.leafHash)
		require.Equal(t, leafHashL, *n.leafHash) // leafHash == HashLeaf(path, value)
		require.Equal(t, leafHashL, n.Hash())    // at height 0, node hash == leaf hash
		require.False(t, n.IsDefaultNode())
		require.True(t, n.VerifyCachedHash())
	})

	// Unrolled independent references for small heights: the empty-sibling placement is spelled out by
	// hand for each branching regime — left (sibling right), right (sibling left), and mixed.
	t.Run("height 1, left branch (pathLeft): sibling on the right", func(t *testing.T) {
		n := NewLeaf(pathLeft, value, 1)
		want := hash.HashInterNode(leafHashL, ledger.GetDefaultHashForHeight(0))
		require.Equal(t, want, n.Hash())
		require.Equal(t, leafHashL, *n.leafHash) // leafHash unchanged by compactification
		require.True(t, n.VerifyCachedHash())
	})

	t.Run("height 1, right branch (pathRight): sibling on the left", func(t *testing.T) {
		n := NewLeaf(pathRight, value, 1)
		want := hash.HashInterNode(ledger.GetDefaultHashForHeight(0), leafHashR)
		require.Equal(t, want, n.Hash())
		require.True(t, n.VerifyCachedHash())
	})

	t.Run("height 2, both right (pathRight)", func(t *testing.T) {
		n := NewLeaf(pathRight, value, 2)
		h1 := hash.HashInterNode(ledger.GetDefaultHashForHeight(0), leafHashR) // height 1: right
		want := hash.HashInterNode(ledger.GetDefaultHashForHeight(1), h1)      // height 2: right
		require.Equal(t, want, n.Hash())
		require.True(t, n.VerifyCachedHash())
	})

	t.Run("height 2, mixed right-then-left (pathMixed)", func(t *testing.T) {
		n := NewLeaf(pathMixed, value, 2)
		h1 := hash.HashInterNode(ledger.GetDefaultHashForHeight(0), leafHashM) // height 1: right (bit 255 = 1)
		want := hash.HashInterNode(h1, ledger.GetDefaultHashForHeight(1))      // height 2: left  (bit 254 = 0)
		require.Equal(t, want, n.Hash())
		require.True(t, n.VerifyCachedHash())
	})

	t.Run("compactified leaf equals fully-expanded interim reference (all branch regimes)", func(t *testing.T) {
		for _, p := range branchRegimePaths {
			for _, height := range []int{1, 9, 256} {
				ref := fullyExpandedAllocatedRef(t, p.path, value, height)
				n := NewLeaf(p.path, value, height)
				require.Equal(t, ref, n.Hash(), "compactified leaf must equal fully-expanded reference: %s @ height %d", p.name, height)
				require.True(t, n.VerifyCachedHash())
			}
		}
	})
}

// Test_NewLeaf_Empty verifies NewLeaf for empty and nil values (an unallocated register); the result
// is the default node whose hash is DefaultHashForHeight(height), independent of path.
func Test_NewLeaf_Empty(t *testing.T) {
	testCases := []struct {
		name  string
		value []byte
	}{
		{"nil value", nil},
		{"empty value", []byte{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, height := range []int{0, 9, 256} {
				n := NewLeaf(pathLeft, tc.value, height)
				require.True(t, n.IsLeaf())
				require.Nil(t, n.leafHash)
				require.Equal(t, ledger.GetDefaultHashForHeight(height), n.Hash())
				require.True(t, n.IsDefaultNode())
				require.Equal(t, pathLeft, *n.Path())
				require.True(t, n.VerifyCachedHash())
			}
		})
	}
}

// Test_NewLeaf_InputImmutability verifies the docstring guarantee that `path` and `value` may be
// mutated by the caller after NewLeaf returns without affecting the constructed node.
func Test_NewLeaf_InputImmutability(t *testing.T) {
	t.Run("allocated leaf", func(t *testing.T) {
		p := testutils.PathByUint16LeftPadded(7)
		v := []byte{0x01, 0x02, 0x03, 0x04}
		n := NewLeaf(p, v, 5)
		hBefore := n.Hash()
		pathBefore := *n.Path()

		p[0] ^= 0xFF // mutate the caller's path array
		p[31] ^= 0xFF
		v[0] ^= 0xFF // mutate the caller's value slice

		require.Equal(t, hBefore, n.Hash(), "node hash must not change when caller mutates inputs")
		require.Equal(t, pathBefore, *n.Path(), "node path must not change when caller mutates inputs")
	})

	t.Run("default leaf", func(t *testing.T) {
		p := testutils.PathByUint16LeftPadded(7)
		n := NewLeaf(p, nil, 5)
		pathBefore := *n.Path()
		p[0] ^= 0xFF
		require.Equal(t, pathBefore, *n.Path())
		require.True(t, n.IsDefaultNode())
	})
}

// ---------------------------------------------------------------------------------------------
// newDefaultLeaf
// ---------------------------------------------------------------------------------------------

// Test_newDefaultLeaf verifies the default-node constructor: nil leafHash, hash == DefaultHashForHeight,
// and path-independence of the hash.
func Test_newDefaultLeaf(t *testing.T) {
	for _, height := range []int{0, 9, 256} {
		n := newDefaultLeaf(pathLeft, height)
		require.True(t, n.IsLeaf())
		require.Nil(t, n.lChild)
		require.Nil(t, n.rChild)
		require.Nil(t, n.leafHash)
		require.Equal(t, height, n.height)
		require.Equal(t, pathLeft, *n.Path())
		require.Equal(t, ledger.GetDefaultHashForHeight(height), n.Hash())
		require.True(t, n.IsDefaultNode())
		require.True(t, n.VerifyCachedHash())
	}

	t.Run("hash is path-independent", func(t *testing.T) {
		a := newDefaultLeaf(pathLeft, 9)
		b := newDefaultLeaf(pathRight, 9)
		require.Equal(t, a.Hash(), b.Hash(), "default-node hash must not depend on path")
	})
}

// ---------------------------------------------------------------------------------------------
// newLeafWithHash
// ---------------------------------------------------------------------------------------------

// Test_newLeafWithHash verifies constructing a leaf from a pre-computed height-0 leaf hash, and that
// it is consistent with NewLeaf (which derives the leaf hash from (path, value) internally).
func Test_newLeafWithHash(t *testing.T) {
	leafHash := hash.HashLeaf(hash.Hash(pathLeft), value)

	t.Run("stores leaf hash and computes node hash", func(t *testing.T) {
		for _, height := range []int{0, 1, 9} {
			n := newLeafWithHash(pathLeft, leafHash, height)
			require.NotNil(t, n.leafHash)
			require.Equal(t, leafHash, *n.leafHash)
			require.Equal(t, ledger.ComputeCompactValueFromLeafHash(hash.Hash(pathLeft), leafHash, height), n.Hash())
			require.Equal(t, pathLeft, *n.Path())
			require.True(t, n.VerifyCachedHash())
		}
	})

	t.Run("height 0 node hash equals the leaf hash", func(t *testing.T) {
		n := newLeafWithHash(pathLeft, leafHash, 0)
		require.Equal(t, leafHash, n.Hash())
	})

	t.Run("consistent with NewLeaf (all branch regimes)", func(t *testing.T) {
		for _, p := range branchRegimePaths {
			lh := hash.HashLeaf(hash.Hash(p.path), value)
			for _, height := range []int{0, 1, 9, 256} {
				viaHash := newLeafWithHash(p.path, lh, height)
				viaValue := NewLeaf(p.path, value, height)
				require.Equal(t, viaValue.Hash(), viaHash.Hash(), "%s @ height %d", p.name, height)
				require.Equal(t, *viaValue.leafHash, *viaHash.leafHash)
			}
		}
	})
}

// ---------------------------------------------------------------------------------------------
// NewRelevelledLeaf
// ---------------------------------------------------------------------------------------------

// Test_NewRelevelledLeaf_Allocated verifies re-levelling an allocated leaf: the height-0 leaf hash is
// preserved, and the re-levelled node equals a leaf freshly constructed at the target height.
//
// Re-levelling depends only on the preserved height-0 leaf hash and the target height, so the result
// must equal a fresh construction at the target REGARDLESS of whether the target is LOWER than, EQUAL
// to, or HIGHER than the origin leaf's own height. We therefore sweep origins at several heights and
// relevel each to targets below, at, and above its own height — across all three branching regimes
// (the target-height fold places empty siblings on path-dependent sides).
func Test_NewRelevelledLeaf_Allocated(t *testing.T) {
	heights := []int{0, 1, 9, 256}
	for _, p := range branchRegimePaths {
		for _, originHeight := range heights {
			origin := NewLeaf(p.path, value, originHeight)
			for _, target := range heights {
				direction := "equal"
				if target < originHeight {
					direction = "down"
				} else if target > originHeight {
					direction = "up"
				}
				t.Run(fmt.Sprintf("%s origin h%d -> target h%d (%s)", p.name, originHeight, target, direction), func(t *testing.T) {
					relevelled := NewRelevelledLeaf(origin, target)
					fresh := NewLeaf(p.path, value, target)
					require.NotNil(t, relevelled.leafHash)
					require.Equal(t, *origin.leafHash, *relevelled.leafHash, "height-0 leaf hash must be preserved")
					require.Equal(t, fresh.Hash(), relevelled.Hash(), "relevel(origin, h) must equal NewLeaf(path,value,h)")
					require.Equal(t, p.path, *relevelled.Path())
					require.True(t, relevelled.VerifyCachedHash())
				})
			}
		}
	}
}

// Test_NewRelevelledLeaf_Default verifies re-levelling an unallocated (default) leaf produces the
// default node at the target height, carrying the SAME register path as the input.
//
// We sweep origins at several heights and relevel each to targets below, at, and above its own height.
// We also sweep all three branching regimes (left/right/mixed): treating the function as a black box, the
// path is an input we must check is preserved, not assume is passed through. This is not redundant — note
// pathLeft is the all-zero path, which aliases ledger.DummyPath, so a bug that dropped the register path
// and substituted DummyPath would go undetected if we only tested pathLeft. The non-zero regimes catch it.
func Test_NewRelevelledLeaf_Default(t *testing.T) {
	heights := []int{0, 1, 9, 256}
	for _, p := range branchRegimePaths {
		for _, originHeight := range heights {
			origin := newDefaultLeaf(p.path, originHeight)
			for _, target := range heights {
				direction := "equal"
				if target < originHeight {
					direction = "down"
				} else if target > originHeight {
					direction = "up"
				}
				t.Run(fmt.Sprintf("%s origin h%d -> target h%d (%s)", p.name, originHeight, target, direction), func(t *testing.T) {
					relevelled := NewRelevelledLeaf(origin, target)
					require.Nil(t, relevelled.leafHash)
					require.Equal(t, ledger.GetDefaultHashForHeight(target), relevelled.Hash())
					require.True(t, relevelled.IsDefaultNode())
					require.Equal(t, p.path, *relevelled.Path(), "register path must be preserved")
					require.True(t, relevelled.VerifyCachedHash())
				})
			}
		}
	}
}

// Test_NewRelevelledLeaf_Nil verifies re-levelling a nil input. A nil represents an empty sub-trie that
// carries no register path. By convention a default leaf must carry the path of the register it represents,
// which a nil input cannot supply, so NewRelevelledLeaf returns nil (an empty sub-trie stays empty),
// regardless of the target height. The "same vs different height" axis does not apply: a nil input carries
// no height of its own.
func Test_NewRelevelledLeaf_Nil(t *testing.T) {
	for _, target := range []int{0, 1, 9, 256} {
		t.Run(fmt.Sprintf("target h%d", target), func(t *testing.T) {
			require.Nil(t, NewRelevelledLeaf(nil, target), "nil input represents an empty sub-trie => nil output")
		})
	}
}

// ---------------------------------------------------------------------------------------------
// NewInterimCompactifiedNode
// ---------------------------------------------------------------------------------------------

// Test_Compactify_EmptySubtrie: both children represent empty sub-tries => the compactified interim
// node is nil (a completely empty sub-trie).
func Test_Compactify_EmptySubtrie(t *testing.T) {
	//      n3
	//    /   \
	// n1(-)  n2(-)
	n1 := NewLeaf(testutils.PathByUint16LeftPadded(0), nil, 4)    // path: ...0000 0000
	n2 := NewLeaf(testutils.PathByUint16LeftPadded(1<<4), nil, 4) // path: ...0001 0000

	t.Run("both children empty", func(t *testing.T) {
		require.Nil(t, NewInterimCompactifiedNode(5, n1, n2))
	})
	t.Run("one child nil and one child empty", func(t *testing.T) {
		require.Nil(t, NewInterimCompactifiedNode(5, nil, n2))
		require.Nil(t, NewInterimCompactifiedNode(5, n1, nil))
	})
	t.Run("both children nil", func(t *testing.T) {
		require.Nil(t, NewInterimCompactifiedNode(5, nil, nil))
	})
}

// Test_Compactify_ToLeaf: one child empty and the other a single allocated register => the
// compactified interim node is a leaf, hash-invariant against the fully-expanded reference.
func Test_Compactify_ToLeaf(t *testing.T) {
	path1 := testutils.PathByUint16LeftPadded(0)      // ...0000 0000
	path2 := testutils.PathByUint16LeftPadded(1 << 4) // ...0001 0000
	valueA := []byte{0x02, 0x02}

	t.Run("left child empty", func(t *testing.T) {
		//      n3
		//    /   \
		// n1(-)  n2(A)
		n1 := NewLeaf(path1, nil, 4)
		n2 := NewLeaf(path2, valueA, 4)
		n3 := NewInterimNode(5, n1, n2)

		nn3 := NewInterimCompactifiedNode(5, n1, n2)
		requireIsLeafWithHash(t, nn3, n3.Hash())
		require.Equal(t, n2.leafHash, nn3.leafHash) // allocated register's leaf hash carried over

		nn3 = NewInterimCompactifiedNode(5, nil, n2)
		requireIsLeafWithHash(t, nn3, n3.Hash())
	})

	t.Run("right child empty", func(t *testing.T) {
		//      n3
		//    /   \
		// n1(A)  n2(-)
		n1 := NewLeaf(path1, valueA, 4)
		n2 := NewLeaf(path2, nil, 4)
		n3 := NewInterimNode(5, n1, n2)

		nn3 := NewInterimCompactifiedNode(5, n1, n2)
		requireIsLeafWithHash(t, nn3, n3.Hash())
		require.Equal(t, n1.leafHash, nn3.leafHash)

		nn3 = NewInterimCompactifiedNode(5, n1, nil)
		requireIsLeafWithHash(t, nn3, n3.Hash())
	})
}

// Test_Compactify_EmptyChild: one child empty and the other holds multiple allocated registers =>
// the empty sub-trie is replaced by nil while the root hash is preserved.
func Test_Compactify_EmptyChild(t *testing.T) {
	valueA := []byte{0x02, 0x02}
	valueB := []byte{0x04, 0x04}

	t.Run("right child empty", func(t *testing.T) {
		//          n5
		//       /     \
		//      n3      n4(-)
		//   /    \
		// n1(A)  n2(B)
		n1 := NewLeaf(testutils.PathByUint16LeftPadded(0), valueA, 4)    // path: ...0000 0000
		n2 := NewLeaf(testutils.PathByUint16LeftPadded(1<<4), valueB, 4) // path: ...0001 0000
		n3 := NewInterimNode(5, n1, n2)
		n4 := NewLeaf(testutils.PathByUint16LeftPadded(3<<4), nil, 5) // path: ...0011 0000
		n5 := NewInterimNode(6, n3, n4)

		nn5 := NewInterimCompactifiedNode(6, n3, n4)
		require.Equal(t, n3, nn5.LeftChild())
		require.Nil(t, nn5.RightChild())
		require.True(t, nn5.VerifyCachedHash())
		require.Equal(t, n5.Hash(), nn5.Hash())
	})

	t.Run("left child empty", func(t *testing.T) {
		//          n5
		//       /     \
		//    n3(-)    n4
		//           /   \
		//        n1(A)  n2(B)
		n1 := NewLeaf(testutils.PathByUint16LeftPadded(2<<4), valueA, 4) // path: ...0010 0000
		n2 := NewLeaf(testutils.PathByUint16LeftPadded(3<<4), valueB, 4) // path: ...0011 0000
		n3 := NewLeaf(testutils.PathByUint16LeftPadded(0), nil, 5)       // path: ...0000 0000
		n4 := NewInterimNode(5, n1, n2)
		n5 := NewInterimNode(6, n3, n4)

		nn5 := NewInterimCompactifiedNode(6, n3, n4)
		require.Nil(t, nn5.LeftChild())
		require.Equal(t, n4, nn5.RightChild())
		require.True(t, nn5.VerifyCachedHash())
		require.Equal(t, n5.Hash(), nn5.Hash())
	})
}

// Test_Compactify_BothChildrenPopulated: both children hold allocated registers => no compactification
// is possible; the result reproduces the full interim node.
func Test_Compactify_BothChildrenPopulated(t *testing.T) {
	//          n5
	//       /     \
	//      n3      n4(C)
	//   /    \
	// n1(A)  n2(B)
	path1 := testutils.PathByUint16LeftPadded(0)      // ...0000 0000
	path2 := testutils.PathByUint16LeftPadded(1 << 4) // ...0001 0000
	path4 := testutils.PathByUint16LeftPadded(3 << 4) // ...0011 0000
	valueA := []byte{0x02, 0x02}
	valueB := []byte{0x03, 0x03}
	valueC := []byte{0x04, 0x04}

	n1 := NewLeaf(path1, valueA, 4)
	n2 := NewLeaf(path2, valueB, 4)
	n3 := NewInterimNode(5, n1, n2)
	n4 := NewLeaf(path4, valueC, 5)
	n5 := NewInterimNode(6, n3, n4)

	nn3 := NewInterimCompactifiedNode(5, n1, n2)
	require.Equal(t, n1, nn3.LeftChild())
	require.Equal(t, n2, nn3.RightChild())
	require.True(t, nn3.VerifyCachedHash())
	require.Equal(t, n3.Hash(), nn3.Hash())

	nn5 := NewInterimCompactifiedNode(6, nn3, n4)
	require.Equal(t, nn3, nn5.LeftChild())
	require.Equal(t, n4, nn5.RightChild())
	require.True(t, nn5.VerifyCachedHash())
	require.Equal(t, n5.Hash(), nn5.Hash())
}

// ---------------------------------------------------------------------------------------------
// Python reference-implementation anchors
// ---------------------------------------------------------------------------------------------

// Test_ReferenceImplementationHashes pins the node hash for fixed (path, value, height) inputs against
// values independently computed by the standalone Python Merkle reference implementation (the source
// of truth): github.com/onflow/flow-internal → reference_implementations/merkle_tree.py. Run it to
// regenerate (its node-level scenario section prints these). A compactified leaf at height h holding a
// single register has the same hash as the root of a height-h subtree containing only it, which is how
// the reference derives these from first principles. A mismatch means this Go implementation and the
// reference disagree (e.g. a systematic change in the hashing primitives or default-hash table). The
// allocated-leaf fixtures are additionally validated by the unrolled / cross-construction tests above;
// refDefaultHeight9 also matches the default-hash-at-height-9 value pinned by the sibling mtrie test
// node.Test_InterimNodeWithoutChildren.
func Test_ReferenceImplementationHashes(t *testing.T) {
	// Values printed by merkle_tree.py's node-level scenario section, one map per branching regime.
	// The reference feeds each path's full 32 bytes into the leaf hash, so pathLeft/pathRight/pathMixed
	// have distinct anchors — independently checking left, right, and mixed folds at the node level.
	refHashes := map[string]map[int]string{
		"pathLeft": {
			0:   "a584b083863e72398b959acd39e39d691f4dee619c28a68f7023a20a69f534cb",
			1:   "357c6c27f26aedc8beaf61b0dd808a509140439eaf5e45a707a5cbf3a891b9ef",
			9:   "0696044ab99fe5afa4c349df6c654db0e40ebdad61f76437d6fb1aca2c2ae89f",
			256: "df9bb7f23235c734d77068f8e90a616a1ea219e0bc9f8910be4a551b158ba222",
		},
		"pathRight": {
			0:   "d7dca2e61b984c07206df415456fdb6be2af390fd526e23ed06b59e877130af9",
			1:   "9cc143de8f84d07bad66f160536a2a9185cc84985a38d43478313d863cb3a48d",
			9:   "40539cfa47c57ca8d82832d0a8472b814ea3c47f5932466e5855abd183ea6bdf",
			256: "ca228d496d4910f0826c12916a564d14a2ed74b74025a88b127cb40641fbdabc",
		},
		"pathMixed": {
			0:   "1b4a471b379d1ab1e1512308fe264dbe44b1a274628654432c1f74828a3c5c0c",
			1:   "e5d47968dafd3c2fac0f88c059854c21d8a8c07dbe369eb581760c31d6fdb154",
			9:   "b6acb1296d222e9a13910ba8bc0c8b7d2caf6b74479e7e94564a6ec66c5fb81a",
			256: "9d3f716355f0f45890a4e62f7fed65b9df32b17446d00b27c8b39b323c82cd8e",
		},
	}
	for _, p := range branchRegimePaths {
		for _, height := range []int{0, 1, 9, 256} {
			n := NewLeaf(p.path, value, height)
			require.Equal(t, refHashes[p.name][height], hashToString(n.Hash()), "reference-hash mismatch for %s at height %d", p.name, height)
		}
	}

	// default node (path-independent); matches node.Test_InterimNodeWithoutChildren at height 9.
	const refDefaultHeight9 = "a37f98dbac56e315fbd4b9f9bc85fbd1b138ed4ae453b128c22c99401495af6d"
	require.Equal(t, refDefaultHeight9, hashToString(newDefaultLeaf(pathLeft, 9).Hash()))
}

// =============================================================================================
// Predicate methods: IsDefaultNode / IsAllocatedRegisterLeaf
// =============================================================================================
//
// INTENT (from node.go):
//   • IsDefaultNode(𝓃)  ≡  "the subtree rooted at 𝓃 holds ZERO allocated registers" (an all-empty /
//     default subtree). Universally applicable: correct for nil, leaves, AND interim nodes,
//     irrespective of compactification or nil-pruning. Mechanism: nil ⇒ true; else
//     hashValue == D(𝒽) where D(𝒽) := GetDefaultHashForHeight(𝒽).
//   • IsAllocatedRegisterLeaf(𝓃): a CHEAP, leaf-only check (reads only leafHash).
//       – for a LEAF 𝓃 (incl. nil):  ≡ ¬IsDefaultNode(𝓃)  ≡  "𝓃 is a leaf holding one allocated register".
//       – for an INTERIM 𝓃:          ALWAYS false — deliberately, even when the interim subtree holds
//         allocated registers. This is the documented divergence from ¬IsDefaultNode on interim nodes
//         (the cheap check under-approximates: it never claims an interim node is a non-default leaf).
//
// CORRECTNESS (why the implementations match their intent — two standing assumptions):
//   (H) collision-resistance of the hash: an all-empty height-𝒽 subtree hashes to D(𝒽) unconditionally,
//       and — only under collision-resistance — the converse holds (hashValue==D(𝒽) ⟹ all-empty). This
//       is the equivalence the struct doc pins and the compaction-model lemma proves by induction.
//   (I) node invariant: hashValue is the correct cached hash, and leafHash==nil for every non-leaf node.
//       Maintained by every constructor except the INSECURE NewNode. (These tests never call NewNode,
//       so both predicates are exercised only on invariant-respecting nodes.)
//
// INDUCTIVE COVERAGE ARGUMENT (why the case set below is exhaustive):
//   The reachable Node space is generated by the constructors; every node is, by structural induction
//   on height 𝒽:
//     BASE (leaves):  nil │ default leaf (leafHash==nil) │ allocated leaf (leafHash≠nil), 𝒽∈{0, >0}.
//     STEP (interim): NewInterimNode(𝒽, L, R) with L,R each nil or a node at height 𝒽-1.
//   IsDefaultNode reads hashValue, and an interim's hashValue is a function of ONLY its two children's
//   hashValues (a nil child contributes D(𝒽-1)). Hence, given the predicate on every height-<𝒽 node,
//   the value at 𝒽 is fixed by whether BOTH children are default. It therefore suffices to cover:
//     – every leaf base case (L0–L3), and
//     – the interim step for each child-emptiness combination: both-empty (I0), one-non-empty (I1),
//       both-non-empty (I2); plus a ≥2-level NESTED case (D/D') to witness that the step composes
//       across depth AND across mixed empty-representations (nil / default-leaf / all-empty-interim) —
//       the "any recursive mix" corollary of the all-empty hash-equivalence lemma.
//   That set exhausts the structural forms the induction ranges over.
//
// Interim test nodes are built with NewInterimNode (NOT NewInterimCompactifiedNode) precisely so the
// node stays an interim node: NewInterimCompactifiedNode would prune/compactify away the very interim
// structure whose predicate behaviour we are verifying.

// Test_IsDefaultNode is the EXHAUSTIVE inductive sweep. Per node it asserts the full node-class
// invariant bundle (via requireDefaultSubtree / requireAllocatedLeaf / requireNonDefaultInterim),
// which checks BOTH predicates together — so this test also fully exercises
// IsAllocatedRegisterLeaf across the case set. Test_IsAllocatedRegisterLeaf below re-covers
// the leaf/interim partition with emphasis on that predicate's deliberate interim under-approximation.
func Test_IsDefaultNode(t *testing.T) {
	t.Run("L0: nil node is default", func(t *testing.T) {
		var n *Node
		requireDefaultSubtree(t, n)
	})

	t.Run("L1: default leaf (unallocated register) is default at any height, path-independent", func(t *testing.T) {
		for _, h := range []int{0, 8, 256} {
			requireDefaultSubtree(t, newDefaultLeaf(pathLeft, h))
			requireDefaultSubtree(t, NewLeaf(pathLeft, nil, h))
			requireDefaultSubtree(t, NewLeaf(pathLeft, []byte{}, h))
			requireDefaultSubtree(t, newDefaultLeaf(pathRight, h)) // path-independence
		}
	})

	t.Run("L2/L3: allocated leaf is NOT default (uncompactified h=0 and compactified h>0)", func(t *testing.T) {
		for _, h := range []int{0, 1, 8, 256} {
			requireAllocatedLeaf(t, NewLeaf(pathLeft, value, h))
		}
	})

	t.Run("I0: all-empty interim node is default (unpruned; interim universality)", func(t *testing.T) {
		//        𝓃 (interim, height 𝒽)        hashValue = H(D(𝒽-1), D(𝒽-1)) = D(𝒽)
		//       ╱   ╲
		//     cL     cR                        cL, cR ∈ {nil, default-leaf}  (all-empty)
		for _, h := range []int{1, 2, 256} {
			require.False(t, emptyInterimNode(h).IsLeaf(), "@h%d must be an interim node", h)
			requireDefaultSubtree(t, emptyInterimNode(h))                                    // both children default-leaf
			requireDefaultSubtree(t, NewInterimNode(h, nil, newDefaultLeaf(pathRight, h-1))) // nil + default-leaf
			requireDefaultSubtree(t, NewInterimNode(h, newDefaultLeaf(pathLeft, h-1), nil))  // default-leaf + nil
		}
	})

	t.Run("I1: interim with exactly one allocated register is NOT default", func(t *testing.T) {
		//        𝓃 (interim, height 𝒽)
		//       ╱   ╲
		//      a     cR                        a = allocated leaf; cR ∈ {nil, default-leaf}
		for _, h := range []int{1, 2, 256} {
			requireNonDefaultInterim(t, oneRegisterInterimNode(h))                                                        // a + nil
			requireNonDefaultInterim(t, NewInterimNode(h, nil, NewLeaf(pathRight, value, h-1)))                           // nil + a
			requireNonDefaultInterim(t, NewInterimNode(h, NewLeaf(pathLeft, value, h-1), newDefaultLeaf(pathRight, h-1))) // a + default-leaf
		}
	})

	t.Run("I2: interim with allocated registers in BOTH children is NOT default", func(t *testing.T) {
		for _, h := range []int{1, 2, 256} {
			requireNonDefaultInterim(t, twoRegisterInterimNode(h))
		}
	})

	t.Run("D: nested all-empty subtree (recursive mix nil / default-leaf / all-empty-interim) is default", func(t *testing.T) {
		//              r (interim, h=2)                all-empty  ⇒  D(2)
		//            ╱          ╲
		//         m (h=1)        d (h=1)               m = all-empty interim, d = default leaf
		//        ╱   ╲
		//      nil    e (h=0)                          e = default leaf
		m := NewInterimNode(1, nil, newDefaultLeaf(pathLeft, 0))
		d := newDefaultLeaf(pathRight, 1)
		r := NewInterimNode(2, m, d)
		require.False(t, r.IsLeaf())
		requireDefaultSubtree(t, r)
	})

	t.Run("D': nested subtree with a single buried allocated register is NOT default", func(t *testing.T) {
		//              r (interim, h=3)                1 allocated register  ⇒  NOT default
		//            ╱          ╲
		//         m3 (h=2)       nil
		//        ╱   ╲
		//      nil    m2 (h=1)
		//            ╱   ╲
		//         a (h=0) nil                          a = allocated leaf
		m2 := NewInterimNode(1, NewLeaf(pathLeft, value, 0), nil)
		m3 := NewInterimNode(2, nil, m2)
		r := NewInterimNode(3, m3, nil)
		requireNonDefaultInterim(t, r)
	})
}

// Test_IsAllocatedRegisterLeaf re-covers the case space partitioned by leaf-vs-interim, with the
// spotlight on this predicate's peculiarities: it recognises allocated leaves at ANY height (compactified
// or not), and it ALWAYS returns false on interim nodes — coinciding with ¬IsDefaultNode on all-empty
// interims but deliberately DIVERGING from it on interims that hold allocated registers.
func Test_IsAllocatedRegisterLeaf(t *testing.T) {
	t.Run("nil ⇒ false", func(t *testing.T) {
		var n *Node
		requireDefaultSubtree(t, n) // bundle asserts IsAllocatedRegisterLeaf()==false
	})

	t.Run("default leaf ⇒ false (any height)", func(t *testing.T) {
		for _, h := range []int{0, 8, 256} {
			requireDefaultSubtree(t, newDefaultLeaf(pathLeft, h))
			requireDefaultSubtree(t, NewLeaf(pathLeft, nil, h))
		}
	})

	t.Run("allocated leaf ⇒ true (uncompactified h=0 and compactified h>0)", func(t *testing.T) {
		requireAllocatedLeaf(t, NewLeaf(pathLeft, value, 0)) // uncompactified
		for _, h := range []int{1, 8, 256} {                 // compactified
			requireAllocatedLeaf(t, NewLeaf(pathLeft, value, h))
		}
	})

	t.Run("interim node ⇒ ALWAYS false, regardless of allocated content", func(t *testing.T) {
		// all-empty interim (also default): false COINCIDES with ¬IsDefaultNode.
		require.False(t, emptyInterimNode(1).IsLeaf())
		requireDefaultSubtree(t, emptyInterimNode(1))

		// interim WITH allocated registers (NOT default): false DIVERGES from ¬IsDefaultNode (=true).
		requireNonDefaultInterim(t, oneRegisterInterimNode(1))
		requireNonDefaultInterim(t, twoRegisterInterimNode(1))
	})
}

// Test_LeafPredicateGuarantee verifies the guarantee stated in node.go: for every LEAF node 𝓃
// (including nil), IsAllocatedRegisterLeaf() == ¬IsDefaultNode().
func Test_LeafPredicateGuarantee(t *testing.T) {
	var nilNode *Node
	leaves := []*Node{
		nilNode,
		newDefaultLeaf(pathLeft, 0),
		newDefaultLeaf(pathLeft, 8),
		newDefaultLeaf(pathLeft, 256),
		NewLeaf(pathLeft, value, 0),
		NewLeaf(pathLeft, value, 8),
		NewLeaf(pathRight, value, 256),
	}
	for i, n := range leaves {
		require.True(t, n.IsLeaf(), "case %d must be a leaf", i)
		require.Equal(t, !n.IsDefaultNode(), n.IsAllocatedRegisterLeaf(), "leaf guarantee violated at case %d", i)
	}
}

// ---------------------------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------------------------

func hashToString(h hash.Hash) string {
	return hex.EncodeToString(h[:])
}

// --- node builders (reuse the pathLeft / pathRight / value fixtures) ------------------------------
// These construct INTERIM nodes of a given emptiness class via NewInterimNode (NOT
// NewInterimCompactifiedNode), so the node stays interim — the shape whose predicate behaviour the
// predicate tests target.

// emptyInterimNode returns an interim node (both children non-nil default leaves) whose subtree is
// entirely unallocated; its hash is D(height).
func emptyInterimNode(height int) *Node {
	return NewInterimNode(height, newDefaultLeaf(pathLeft, height-1), newDefaultLeaf(pathRight, height-1))
}

// oneRegisterInterimNode returns an interim node holding exactly one allocated register (allocated leaf
// on the left, empty right child).
func oneRegisterInterimNode(height int) *Node {
	return NewInterimNode(height, NewLeaf(pathLeft, value, height-1), nil)
}

// twoRegisterInterimNode returns an interim node holding two allocated registers (one per child).
func twoRegisterInterimNode(height int) *Node {
	return NewInterimNode(height, NewLeaf(pathLeft, value, height-1), NewLeaf(pathRight, value, height-1))
}

// --- node-class invariant assertions --------------------------------------------------------------
// Each names one of the three node classes the predicate analysis revolves around and asserts the
// FULL invariant bundle for that class from BOTH predicates plus structure. Extracted because the
// bundle recurs across every predicate case.

// requireDefaultSubtree asserts that 𝓃 represents an all-empty (default) subtree, for ANY
// representation (nil, default leaf, or all-empty interim node): IsDefaultNode is true and the cheap
// leaf-check is false; a non-nil node's cached hash is the height default and internally consistent.
func requireDefaultSubtree(t *testing.T, n *Node) {
	t.Helper()
	require.True(t, n.IsDefaultNode(), "expected an all-empty (default) subtree")
	require.False(t, n.IsAllocatedRegisterLeaf(), "a default subtree is never a non-default leaf")
	if n != nil {
		require.True(t, n.VerifyCachedHash())
		require.Equal(t, ledger.GetDefaultHashForHeight(n.height), n.Hash())
	}
}

// requireAllocatedLeaf asserts that 𝓃 is a leaf holding exactly one allocated register: it is a leaf,
// not default, and the cheap leaf-check recognises it.
func requireAllocatedLeaf(t *testing.T, n *Node) {
	t.Helper()
	require.True(t, n.IsLeaf(), "expected a leaf")
	require.False(t, n.IsDefaultNode(), "an allocated leaf is not default")
	require.True(t, n.IsAllocatedRegisterLeaf(), "cheap leaf-check must recognise an allocated leaf")
	require.True(t, n.VerifyCachedHash())
}

// requireNonDefaultInterim asserts that 𝓃 is an interim node holding ≥1 allocated register: it is NOT a
// leaf, not default, and the cheap leaf-check deliberately returns false (under-approximation on interim
// nodes — the documented divergence from ¬IsDefaultNode).
func requireNonDefaultInterim(t *testing.T, n *Node) {
	t.Helper()
	require.False(t, n.IsLeaf(), "expected an interim node")
	require.False(t, n.IsDefaultNode(), "expected a non-default (allocated) subtree")
	require.False(t, n.IsAllocatedRegisterLeaf(), "cheap leaf-check must return false on interim nodes")
	require.True(t, n.VerifyCachedHash())
}

// fullyExpandedAllocatedRef builds the hash of the height-`height` subtree holding the single
// allocated register (path, value) by explicitly composing interim nodes with default (empty)
// siblings, using NewInterimNode. This is the fully-expanded (perfect-trie) reference against which
// a compactified leaf's hash must be invariant. The register's leaf sits on the side selected by the
// path bit at each level, matching the fully-expanded trie geometry.
func fullyExpandedAllocatedRef(t *testing.T, path ledger.Path, value []byte, height int) hash.Hash {
	t.Helper()
	node := NewLeaf(path, value, 0) // height-0 (uncompactified) leaf
	for h := 1; h <= height; h++ {
		bit := readPathBit(path, ledger.NodeMaxHeight-h)
		if bit == 1 { // register branches right => empty sibling on the left
			node = NewInterimNode(h, nil, node)
		} else { // register branches left => empty sibling on the right
			node = NewInterimNode(h, node, nil)
		}
	}
	return node.Hash()
}

// readPathBit returns the bit of `path` at position `index` (0 == MSB of byte 0).
func readPathBit(path ledger.Path, index int) int {
	return int((path[index>>3] >> (7 - (index & 7))) & 1)
}

// requireIsLeafWithHash verifies that `n` is a leaf node whose hash equals `expectedHash`.
func requireIsLeafWithHash(t *testing.T, n *Node, expectedHash hash.Hash) {
	t.Helper()
	require.Nil(t, n.LeftChild())
	require.Nil(t, n.RightChild())
	require.Equal(t, expectedHash, n.Hash())
	require.True(t, n.VerifyCachedHash())
	require.True(t, n.IsLeaf())
}
