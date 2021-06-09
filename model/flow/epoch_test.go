package flow_test

import (
	"testing"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/require"
)

func TestClusterQCVoteData_Equality(t *testing.T) {

	pks := unittest.PublicKeysFixture(2, crypto.BLSBLS12381)

	_ = len(pks)

	t.Run("empty structures are equal", func(t *testing.T) {
		a := &flow.ClusterQCVoteData{}
		b := &flow.ClusterQCVoteData{}
		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})
}

//	t.Run("sig data triggers", func(t *testing.T) {
//		a := &ClusterQCVoteData{
//			SigData:  []byte{1, 2},
//			VoterIDs: nil,
//		}
//		b := &ClusterQCVoteData{
//			SigData:  []byte{1, 3},
//			VoterIDs: nil,
//		}
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("VoterID len difference triggers", func(t *testing.T) {
//		a := &ClusterQCVoteData{
//			SigData:  nil,
//			VoterIDs: []Identifier{HashToID([]byte{1, 2, 3})},
//		}
//		b := &ClusterQCVoteData{
//			SigData:  nil,
//			VoterIDs: []Identifier{},
//		}
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("VoterID len values triggers", func(t *testing.T) {
//		a := &ClusterQCVoteData{
//			SigData:  nil,
//			VoterIDs: []Identifier{HashToID([]byte{1, 2, 3})},
//		}
//		b := &ClusterQCVoteData{
//			SigData:  nil,
//			VoterIDs: []Identifier{HashToID([]byte{3, 2, 1})},
//		}
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("filled structures match with same data", func(t *testing.T) {
//		a := &ClusterQCVoteData{
//			SigData:  []byte{3, 3, 3},
//			VoterIDs: []Identifier{HashToID([]byte{1, 2, 3}), HashToID([]byte{3, 2, 1})},
//		}
//		b := &ClusterQCVoteData{
//			SigData:  []byte{3, 3, 3},
//			VoterIDs: []Identifier{HashToID([]byte{1, 2, 3}), HashToID([]byte{3, 2, 1})},
//		}
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//}
//
//func TestEpochCommit_EqualTo(t *testing.T) {
//
//	qcA := ClusterQCVoteData{
//		SigData:  []byte{3, 3, 3},
//		VoterIDs: []Identifier{HashToID([]byte{1, 2, 3}), HashToID([]byte{3, 2, 1})},
//	}
//
//	qcB := ClusterQCVoteData{
//		SigData:  []byte{1, 1, 1},
//		VoterIDs: []Identifier{HashToID([]byte{1, 2, 3}), HashToID([]byte{3, 2, 1})},
//	}
//
//	pks := unittest.PublicKeysFixture(2,  crypto.BLSBLS12381)
//
//
//	t.Run("empty are equal", func(t *testing.T) {
//		a := &EpochCommit{}
//		b := &EpochCommit{}
//
//		require.True(t, a.EqualTo(b))
//		require.True(t, b.EqualTo(a))
//	})
//
//	t.Run("counter diff", func(t *testing.T) {
//		a := &EpochCommit{Counter: 1}
//		b := &EpochCommit{Counter: 2}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("ClusterQCs diffs", func(t *testing.T) {
//		a := &EpochCommit{ClusterQCs: []ClusterQCVoteData{qcA}}
//		b := &EpochCommit{ClusterQCs: []ClusterQCVoteData{qcB}}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("ClusterQCs different lengths", func(t *testing.T) {
//		a := &EpochCommit{ClusterQCs: []ClusterQCVoteData{qcA}}
//		b := &EpochCommit{ClusterQCs: []ClusterQCVoteData{}}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("ClusterQCs same", func(t *testing.T) {
//
//		a := &EpochCommit{ClusterQCs: []ClusterQCVoteData{qcA}}
//		b := &EpochCommit{ClusterQCs: []ClusterQCVoteData{qcA}}
//
//		require.True(t, a.EqualTo(b))
//		require.True(t, b.EqualTo(a))
//	})
//
//
//	t.Run("DKGGroupKey same", func(t *testing.T) {
//
//		a := &EpochCommit{DKGGroupKey: pks[1]}
//		b := &EpochCommit{DKGGroupKey: pks[1]}
//
//		require.True(t, a.EqualTo(b))
//		require.True(t, b.EqualTo(a))
//	})
//
//	t.Run("DKGGroupKey diff", func(t *testing.T) {
//
//		a := &EpochCommit{DKGGroupKey: pks[1]}
//		b := &EpochCommit{DKGGroupKey: pks[0]}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("DKGParticipantKeys different length", func(t *testing.T) {
//
//		a := &EpochCommit{DKGParticipantKeys: []crypto.PublicKey{}}
//		b := &EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[0]}}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("DKGParticipantKeys different data", func(t *testing.T) {
//
//		a := &EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[1]}}
//		b := &EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[0]}}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("DKGParticipantKeys same data", func(t *testing.T) {
//
//		a := &EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[1]}}
//		b := &EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[1]}}
//
//		require.True(t, a.EqualTo(b))
//		require.True(t, b.EqualTo(a))
//	})
//}
//
//func TestEpochSetup_EqualTo(t *testing.T) {
//
//	identityA := unittest.IdentityFixture()
//	identityB := unittest.IdentityFixture()
//
//	assignmentA := AssignmentList{[]Identifier{[32]byte{1, 2, 3}, [32]byte{2, 2, 2}}}
//	assignmentB := AssignmentList{[]Identifier{[32]byte{1, 2, 3}, [32]byte{}}}
//
//	t.Run("empty are the same", func(t *testing.T) {
//		a := &EpochSetup{}
//		b := &EpochSetup{}
//
//		require.True(t, a.EqualTo(b))
//		require.True(t, b.EqualTo(a))
//	})
//
//	t.Run("Counter", func(t *testing.T) {
//		a := &EpochSetup{Counter: 1}
//		b := &EpochSetup{Counter: 2}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("FirstView", func(t *testing.T) {
//		a := &EpochSetup{FirstView: 1}
//		b := &EpochSetup{FirstView: 2}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("DKGPhase1FinalView", func(t *testing.T) {
//		a := &EpochSetup{DKGPhase1FinalView: 1}
//		b := &EpochSetup{DKGPhase1FinalView: 2}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("DKGPhase2FinalView", func(t *testing.T) {
//		a := &EpochSetup{DKGPhase2FinalView: 1}
//		b := &EpochSetup{DKGPhase2FinalView: 2}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("DKGPhase3FinalView", func(t *testing.T) {
//		a := &EpochSetup{DKGPhase3FinalView: 1}
//		b := &EpochSetup{DKGPhase3FinalView: 2}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("FinalView", func(t *testing.T) {
//		a := &EpochSetup{FinalView: 1}
//		b := &EpochSetup{FinalView: 2}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("Participants length differ", func(t *testing.T) {
//
//		a := &EpochSetup{Participants: IdentityList{identityA}}
//		b := &EpochSetup{Participants: IdentityList{}}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("Participants length same but different data", func(t *testing.T) {
//
//		a := &EpochSetup{Participants: IdentityList{identityA}}
//		b := &EpochSetup{Participants: IdentityList{identityB}}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("Participants length same with same data", func(t *testing.T) {
//
//		a := &EpochSetup{Participants: IdentityList{identityA}}
//		b := &EpochSetup{Participants: IdentityList{identityA}}
//
//		require.True(t, a.EqualTo(b))
//		require.True(t, b.EqualTo(a))
//	})
//
//	t.Run("Assignments different", func(t *testing.T) {
//
//		a := &EpochSetup{Assignments: assignmentA}
//		b := &EpochSetup{Assignments: assignmentB}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//
//	t.Run("Assignments same", func(t *testing.T) {
//
//		a := &EpochSetup{Assignments: assignmentB}
//		b := &EpochSetup{Assignments: assignmentB}
//
//		require.True(t, a.EqualTo(b))
//		require.True(t, b.EqualTo(a))
//	})
//
//	t.Run("RandomSource same", func(t *testing.T) {
//
//		a := &EpochSetup{RandomSource: []byte{1, 2, 3}}
//		b := &EpochSetup{RandomSource: []byte{1, 2, 3}}
//
//		require.True(t, a.EqualTo(b))
//		require.True(t, b.EqualTo(a))
//	})
//
//	t.Run("RandomSource diff", func(t *testing.T) {
//
//		a := &EpochSetup{RandomSource: []byte{1, 2, 3}}
//		b := &EpochSetup{RandomSource: []byte{1}}
//
//		require.False(t, a.EqualTo(b))
//		require.False(t, b.EqualTo(a))
//	})
//}
