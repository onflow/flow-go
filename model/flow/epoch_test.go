package flow_test

import (
	"math/rand"
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMalleability performs sanity checks to ensure that epoch related entities are not malleable.
func TestMalleability(t *testing.T) {
	t.Run("EpochSetup", func(t *testing.T) {
		unittest.RequireEntityNonMalleable(t, unittest.EpochSetupFixture())
	})
	t.Run("EpochCommit with nil DKGIndexMap", func(t *testing.T) {
		checker := unittest.NewMalleabilityChecker(unittest.WithPinnedField("DKGIndexMap"))
		// Due to `DKGIndexMap` being nil, `MalleabilityChecker` will skip mutating this field.
		err := checker.Check(unittest.EpochCommitFixture())
		require.NoError(t, err)
	})

	t.Run("EpochCommit with proper DKGIndexMap", func(t *testing.T) {
		checker := unittest.NewMalleabilityChecker(unittest.WithFieldGenerator("DKGIndexMap", func() flow.DKGIndexMap {
			return flow.DKGIndexMap{unittest.IdentifierFixture(): 0, unittest.IdentifierFixture(): 1}
		}))
		err := checker.Check(unittest.EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.DKGIndexMap = flow.DKGIndexMap{unittest.IdentifierFixture(): 0, unittest.IdentifierFixture(): 1}
		}))
		require.NoError(t, err)
	})
	t.Run("EpochRecover", func(t *testing.T) {
		checker := unittest.NewMalleabilityChecker(unittest.WithFieldGenerator("EpochCommit.DKGIndexMap", func() flow.DKGIndexMap {
			return flow.DKGIndexMap{unittest.IdentifierFixture(): 0, unittest.IdentifierFixture(): 1}
		}))
		err := checker.Check(unittest.EpochRecoverFixture())
		require.NoError(t, err)
	})

	epochExtensionFixture := func() flow.EpochExtension {
		firstView := rand.Uint64()
		return flow.EpochExtension{
			FirstView: firstView,
			FinalView: firstView + 10,
		}
	}

	epochStateContainerFixture := func() *flow.EpochStateContainer {
		return &flow.EpochStateContainer{
			SetupID:          unittest.EpochSetupFixture().ID(),
			CommitID:         unittest.EpochCommitFixture().ID(),
			ActiveIdentities: unittest.DynamicIdentityEntryListFixture(5),
			EpochExtensions:  []flow.EpochExtension{epochExtensionFixture()},
		}
	}

	t.Run("EpochStateContainer with nil EpochExtensions", func(t *testing.T) {
		checker := unittest.NewMalleabilityChecker(
			unittest.WithPinnedField("EpochExtensions"),
			unittest.WithFieldGenerator("ActiveIdentities", func() flow.DynamicIdentityEntryList {
				return flow.DynamicIdentityEntryList{
					{
						NodeID:  unittest.IdentifierFixture(),
						Ejected: rand.Intn(2) == 1,
					},
				}
			}),
		)

		// Due to `EpochExtensions` being nil, `MalleabilityChecker` will skip mutating this field.
		err := checker.Check(epochStateContainerFixture())
		require.NoError(t, err)
	})

	t.Run("EpochStateContainer with nil ActiveIdentities", func(t *testing.T) {
		checker := unittest.NewMalleabilityChecker(
			unittest.WithFieldGenerator("EpochExtensions", func() []flow.EpochExtension { return []flow.EpochExtension{epochExtensionFixture()} }),
			unittest.WithPinnedField("ActiveIdentities"),
		)

		// Due to `ActiveIdentities` being nil, `MalleabilityChecker` will skip mutating this field.
		err := checker.Check(epochStateContainerFixture())
		require.NoError(t, err)
	})

	t.Run("EpochStateContainer", func(t *testing.T) {
		checker := unittest.NewMalleabilityChecker(
			unittest.WithFieldGenerator("EpochExtensions", func() []flow.EpochExtension { return []flow.EpochExtension{epochExtensionFixture()} }),
			unittest.WithFieldGenerator("ActiveIdentities", func() flow.DynamicIdentityEntryList {
				return flow.DynamicIdentityEntryList{
					{
						NodeID:  unittest.IdentifierFixture(),
						Ejected: rand.Intn(2) == 1,
					},
				}
			}),
		)

		err := checker.Check(epochStateContainerFixture())
		require.NoError(t, err)
	})

	t.Run("MinEpochStateEntry", func(t *testing.T) {
		checker := unittest.NewMalleabilityChecker(
			unittest.WithFieldGenerator("PreviousEpoch", func() flow.EpochStateContainer { return *epochStateContainerFixture() }),
			unittest.WithFieldGenerator("CurrentEpoch", func() flow.EpochStateContainer { return *epochStateContainerFixture() }),
			unittest.WithFieldGenerator("NextEpoch", func() flow.EpochStateContainer { return *epochStateContainerFixture() }),
		)

		err := checker.Check(unittest.EpochStateFixture(unittest.WithNextEpochProtocolState()).MinEpochStateEntry)
		require.NoError(t, err)
	})
}

func TestClusterQCVoteData_Equality(t *testing.T) {
	pks := unittest.PublicKeysFixture(2, crypto.BLSBLS12381)
	_ = len(pks)

	t.Run("empty structures are equal", func(t *testing.T) {
		a := &flow.ClusterQCVoteData{}
		b := &flow.ClusterQCVoteData{}
		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("sig data triggers", func(t *testing.T) {
		a := &flow.ClusterQCVoteData{
			SigData:  []byte{1, 2},
			VoterIDs: nil,
		}
		b := &flow.ClusterQCVoteData{
			SigData:  []byte{1, 3},
			VoterIDs: nil,
		}
		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("VoterID len difference triggers", func(t *testing.T) {
		a := &flow.ClusterQCVoteData{
			SigData:  nil,
			VoterIDs: []flow.Identifier{flow.HashToID([]byte{1, 2, 3})},
		}
		b := &flow.ClusterQCVoteData{
			SigData:  nil,
			VoterIDs: []flow.Identifier{},
		}
		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("VoterID len values triggers", func(t *testing.T) {
		a := &flow.ClusterQCVoteData{
			SigData:  nil,
			VoterIDs: []flow.Identifier{flow.HashToID([]byte{1, 2, 3})},
		}
		b := &flow.ClusterQCVoteData{
			SigData:  nil,
			VoterIDs: []flow.Identifier{flow.HashToID([]byte{3, 2, 1})},
		}
		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("filled structures match with same data", func(t *testing.T) {
		a := &flow.ClusterQCVoteData{
			SigData:  []byte{3, 3, 3},
			VoterIDs: []flow.Identifier{flow.HashToID([]byte{1, 2, 3}), flow.HashToID([]byte{3, 2, 1})},
		}
		b := &flow.ClusterQCVoteData{
			SigData:  []byte{3, 3, 3},
			VoterIDs: []flow.Identifier{flow.HashToID([]byte{1, 2, 3}), flow.HashToID([]byte{3, 2, 1})},
		}
		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})
}

func TestEpochCommit_EqualTo(t *testing.T) {

	qcA := flow.ClusterQCVoteData{
		SigData:  []byte{3, 3, 3},
		VoterIDs: []flow.Identifier{flow.HashToID([]byte{1, 2, 3}), flow.HashToID([]byte{3, 2, 1})},
	}

	qcB := flow.ClusterQCVoteData{
		SigData:  []byte{1, 1, 1},
		VoterIDs: []flow.Identifier{flow.HashToID([]byte{1, 2, 3}), flow.HashToID([]byte{3, 2, 1})},
	}

	pks := unittest.PublicKeysFixture(2, crypto.BLSBLS12381)

	t.Run("empty are equal", func(t *testing.T) {
		a := &flow.EpochCommit{}
		b := &flow.EpochCommit{}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("counter diff", func(t *testing.T) {
		a := &flow.EpochCommit{Counter: 1}
		b := &flow.EpochCommit{Counter: 2}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("ClusterQCs diffs", func(t *testing.T) {
		a := &flow.EpochCommit{ClusterQCs: []flow.ClusterQCVoteData{qcA}}
		b := &flow.EpochCommit{ClusterQCs: []flow.ClusterQCVoteData{qcB}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("ClusterQCs different lengths", func(t *testing.T) {
		a := &flow.EpochCommit{ClusterQCs: []flow.ClusterQCVoteData{qcA}}
		b := &flow.EpochCommit{ClusterQCs: []flow.ClusterQCVoteData{}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("ClusterQCs same", func(t *testing.T) {

		a := &flow.EpochCommit{ClusterQCs: []flow.ClusterQCVoteData{qcA}}
		b := &flow.EpochCommit{ClusterQCs: []flow.ClusterQCVoteData{qcA}}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("DKGGroupKey same", func(t *testing.T) {

		a := &flow.EpochCommit{DKGGroupKey: pks[1]}
		b := &flow.EpochCommit{DKGGroupKey: pks[1]}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("DKGGroupKey diff", func(t *testing.T) {

		a := &flow.EpochCommit{DKGGroupKey: pks[1]}
		b := &flow.EpochCommit{DKGGroupKey: pks[0]}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("DKGParticipantKeys different length", func(t *testing.T) {

		a := &flow.EpochCommit{DKGParticipantKeys: []crypto.PublicKey{}}
		b := &flow.EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[0]}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("DKGParticipantKeys different data", func(t *testing.T) {

		a := &flow.EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[1]}}
		b := &flow.EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[0]}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("DKGParticipantKeys same data", func(t *testing.T) {

		a := &flow.EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[1]}}
		b := &flow.EpochCommit{DKGParticipantKeys: []crypto.PublicKey{pks[1]}}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("DKGData different length", func(t *testing.T) {

		a := &flow.EpochCommit{DKGIndexMap: flow.DKGIndexMap{flow.HashToID([]byte{1}): 1}}
		b := &flow.EpochCommit{DKGIndexMap: flow.DKGIndexMap{}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("DKGData different data", func(t *testing.T) {

		a := &flow.EpochCommit{DKGIndexMap: flow.DKGIndexMap{flow.HashToID([]byte{1}): 1}}
		b := &flow.EpochCommit{DKGIndexMap: flow.DKGIndexMap{flow.HashToID([]byte{1}): 2}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("DKGData different data - zero value", func(t *testing.T) {

		a := &flow.EpochCommit{DKGIndexMap: flow.DKGIndexMap{flow.HashToID([]byte{1}): 0}}
		b := &flow.EpochCommit{DKGIndexMap: flow.DKGIndexMap{flow.HashToID([]byte{2}): 1}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("DKGData  same data", func(t *testing.T) {

		a := &flow.EpochCommit{DKGIndexMap: flow.DKGIndexMap{flow.HashToID([]byte{1, 2, 3}): 1}}
		b := &flow.EpochCommit{DKGIndexMap: flow.DKGIndexMap{flow.HashToID([]byte{1, 2, 3}): 1}}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})
}

func TestEpochSetup_EqualTo(t *testing.T) {

	identityA := &unittest.IdentityFixture().IdentitySkeleton
	identityB := &unittest.IdentityFixture().IdentitySkeleton

	assignmentA := flow.AssignmentList{[]flow.Identifier{[32]byte{1, 2, 3}, [32]byte{2, 2, 2}}}
	assignmentB := flow.AssignmentList{[]flow.Identifier{[32]byte{1, 2, 3}, [32]byte{}}}

	t.Run("empty are the same", func(t *testing.T) {
		a := &flow.EpochSetup{}
		b := &flow.EpochSetup{}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("Counter", func(t *testing.T) {
		a := &flow.EpochSetup{Counter: 1}
		b := &flow.EpochSetup{Counter: 2}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("FirstView", func(t *testing.T) {
		a := &flow.EpochSetup{FirstView: 1}
		b := &flow.EpochSetup{FirstView: 2}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("DKGPhase1FinalView", func(t *testing.T) {
		a := &flow.EpochSetup{DKGPhase1FinalView: 1}
		b := &flow.EpochSetup{DKGPhase1FinalView: 2}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("DKGPhase2FinalView", func(t *testing.T) {
		a := &flow.EpochSetup{DKGPhase2FinalView: 1}
		b := &flow.EpochSetup{DKGPhase2FinalView: 2}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("DKGPhase3FinalView", func(t *testing.T) {
		a := &flow.EpochSetup{DKGPhase3FinalView: 1}
		b := &flow.EpochSetup{DKGPhase3FinalView: 2}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("FinalView", func(t *testing.T) {
		a := &flow.EpochSetup{FinalView: 1}
		b := &flow.EpochSetup{FinalView: 2}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Participants length differ", func(t *testing.T) {

		a := &flow.EpochSetup{Participants: flow.IdentitySkeletonList{identityA}}
		b := &flow.EpochSetup{Participants: flow.IdentitySkeletonList{}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Participants length same but different data", func(t *testing.T) {

		a := &flow.EpochSetup{Participants: flow.IdentitySkeletonList{identityA}}
		b := &flow.EpochSetup{Participants: flow.IdentitySkeletonList{identityB}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Participants length same with same data", func(t *testing.T) {

		a := &flow.EpochSetup{Participants: flow.IdentitySkeletonList{identityA}}
		b := &flow.EpochSetup{Participants: flow.IdentitySkeletonList{identityA}}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("Assignments different", func(t *testing.T) {

		a := &flow.EpochSetup{Assignments: assignmentA}
		b := &flow.EpochSetup{Assignments: assignmentB}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Assignments same", func(t *testing.T) {

		a := &flow.EpochSetup{Assignments: assignmentB}
		b := &flow.EpochSetup{Assignments: assignmentB}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("RandomSource same", func(t *testing.T) {

		a := &flow.EpochSetup{RandomSource: []byte{1, 2, 3}}
		b := &flow.EpochSetup{RandomSource: []byte{1, 2, 3}}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("RandomSource diff", func(t *testing.T) {

		a := &flow.EpochSetup{RandomSource: []byte{1, 2, 3}}
		b := &flow.EpochSetup{RandomSource: []byte{1}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})
}
