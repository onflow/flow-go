package flow_test

import (
	"testing"
	"time"

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
		require.Nil(t, unittest.EpochCommitFixture().DKGIndexMap) // sanity check that the fixture has left `DKGIndexMap` nil
		unittest.RequireEntityNonMalleable(t, unittest.EpochCommitFixture(),
			// We pin the `DKGIndexMap` to the current value (nil), so `MalleabilityChecker` will not mutate this field:
			unittest.WithPinnedField("DKGIndexMap"),
		)
	})

	t.Run("EpochCommit with proper DKGIndexMap", func(t *testing.T) {
		checker := unittest.NewMalleabilityChecker(unittest.WithFieldGenerator("DKGIndexMap", func() flow.DKGIndexMap {
			return flow.DKGIndexMap{unittest.IdentifierFixture(): 0, unittest.IdentifierFixture(): 1}
		}))
		err := checker.CheckEntity(unittest.EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.DKGIndexMap = flow.DKGIndexMap{unittest.IdentifierFixture(): 0, unittest.IdentifierFixture(): 1}
		}))
		require.NoError(t, err)
	})
	t.Run("EpochRecover", func(t *testing.T) {
		checker := unittest.NewMalleabilityChecker(unittest.WithFieldGenerator("EpochCommit.DKGIndexMap", func() flow.DKGIndexMap {
			return flow.DKGIndexMap{unittest.IdentifierFixture(): 0, unittest.IdentifierFixture(): 1}
		}))
		err := checker.CheckEntity(unittest.EpochRecoverFixture())
		require.NoError(t, err)
	})

	t.Run("EpochStateContainer", func(t *testing.T) {
		unittest.RequireEntityNonMalleable(t, unittest.EpochStateContainerFixture())
	})

	t.Run("MinEpochStateEntry", func(t *testing.T) {
		unittest.RequireEntityNonMalleable(t, unittest.EpochStateFixture(unittest.WithNextEpochProtocolState()).MinEpochStateEntry)
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

// TestNewEpochSetup verifies the behavior of the NewEpochSetup constructor function.
// It checks for correct handling of both valid and invalid inputs.
//
// Test Cases:
//
// 1. Valid input returns setup:
//   - Ensures that providing all required and correctly formatted fields results in a successful creation of an EpochSetup instance.
//
// 2. Invalid FirstView and FinalView:
//   - Verifies that an error is returned when FirstView is not less than FinalView, as this violates the expected chronological order.
//
// 3. Invalid FirstView >= DKGPhase1FinalView:
//   - Ensures that FirstView must end before DKG Phase 1 ends.
//
// 4. Invalid DKGPhase1FinalView >= DKGPhase2FinalView:
//   - Ensures DKG Phase 1 must end before DKG Phase 2 ends.
//
// 5. Invalid DKGPhase2FinalView >= DKGPhase3FinalView:
//   - Ensures DKG Phase 2 must end before DKG Phase 3 ends.
//
// 6. Invalid DKGPhase3FinalView >= FinalView:
//   - Ensures DKG Phase 3 must end before FinalView.
//
// 7. Invalid participants:
//   - Checks that an error is returned when the Participants field is nil.
//
// 8. Invalid assignments:
//   - Ensures that an error is returned when the Assignments field is nil.
//
// 9. Invalid RandomSource:
//   - Validates that an error is returned when the RandomSource does not meet the required length.
//
// 10. Invalid TargetDuration:
//   - Confirms that an error is returned when TargetDuration is zero.
func TestNewEpochSetup(t *testing.T) {
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	validParticipants := participants.Sort(flow.Canonical[flow.Identity]).ToSkeleton()
	validRandomSource := unittest.SeedFixture(flow.EpochSetupRandomSourceLength)
	validAssignments := unittest.ClusterAssignment(1, validParticipants)

	t.Run("valid input", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			Counter:            1,
			FirstView:          10,
			DKGPhase1FinalView: 20,
			DKGPhase2FinalView: 30,
			DKGPhase3FinalView: 40,
			FinalView:          50,
			Participants:       validParticipants,
			Assignments:        validAssignments,
			RandomSource:       validRandomSource,
			TargetDuration:     60 * 60,
			TargetEndTime:      uint64(time.Now().Unix()) + 1000,
		}

		setup, err := flow.NewEpochSetup(untrusted)
		require.NoError(t, err)
		require.NotNil(t, setup)
	})

	t.Run("invalid FirstView and FinalView", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			FirstView:          100,
			FinalView:          90,
			DKGPhase1FinalView: 20,
			DKGPhase2FinalView: 30,
			DKGPhase3FinalView: 40,
			Participants:       validParticipants,
			Assignments:        validAssignments,
			RandomSource:       validRandomSource,
		}
		setup, err := flow.NewEpochSetup(untrusted)
		require.Error(t, err)
		require.Nil(t, setup)
		require.Contains(t, err.Error(), "invalid timing - first view (100) ends after the final view (90)")
	})

	t.Run("invalid FirstView >= DKGPhase1FinalView", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			FirstView:          20,
			DKGPhase1FinalView: 10,
			DKGPhase2FinalView: 30,
			DKGPhase3FinalView: 40,
			FinalView:          50,
			Participants:       validParticipants,
			Assignments:        validAssignments,
			RandomSource:       validRandomSource,
			TargetDuration:     60,
		}
		setup, err := flow.NewEpochSetup(untrusted)
		require.Error(t, err)
		require.Nil(t, setup)
		require.Contains(t, err.Error(), "invalid timing - first view (20) ends after dkg phase 1 (10)")
	})

	t.Run("invalid DKGPhase1FinalView >= DKGPhase2FinalView", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			FirstView:          10,
			DKGPhase1FinalView: 30,
			DKGPhase2FinalView: 20,
			DKGPhase3FinalView: 40,
			FinalView:          50,
			Participants:       validParticipants,
			Assignments:        validAssignments,
			RandomSource:       validRandomSource,
			TargetDuration:     60,
		}
		setup, err := flow.NewEpochSetup(untrusted)
		require.Error(t, err)
		require.Nil(t, setup)
		require.Contains(t, err.Error(), "invalid dkg timing - phase 1 (30) ends after phase 2 (20)")
	})

	t.Run("invalid DKGPhase2FinalView >= DKGPhase3FinalView", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			FirstView:          10,
			DKGPhase1FinalView: 20,
			DKGPhase2FinalView: 40,
			DKGPhase3FinalView: 30,
			FinalView:          50,
			Participants:       validParticipants,
			Assignments:        validAssignments,
			RandomSource:       validRandomSource,
			TargetDuration:     60,
		}
		setup, err := flow.NewEpochSetup(untrusted)
		require.Error(t, err)
		require.Nil(t, setup)
		require.Contains(t, err.Error(), "invalid dkg timing - phase 2 (40) ends after phase 3 (30)")
	})

	t.Run("invalid DKGPhase3FinalView >= FinalView", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			FirstView:          10,
			DKGPhase1FinalView: 20,
			DKGPhase2FinalView: 30,
			DKGPhase3FinalView: 60,
			FinalView:          50,
			Participants:       validParticipants,
			Assignments:        validAssignments,
			RandomSource:       validRandomSource,
			TargetDuration:     60,
		}
		setup, err := flow.NewEpochSetup(untrusted)
		require.Error(t, err)
		require.Nil(t, setup)
		require.Contains(t, err.Error(), "invalid timing - dkg phase 3 (60) ends after final view (50)")
	})

	t.Run("invalid participants", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			FirstView:          10,
			DKGPhase1FinalView: 20,
			DKGPhase2FinalView: 30,
			DKGPhase3FinalView: 40,
			FinalView:          50,
			Participants:       nil,
			Assignments:        validAssignments,
			RandomSource:       validRandomSource,
		}
		setup, err := flow.NewEpochSetup(untrusted)
		require.Error(t, err)
		require.Nil(t, setup)
		require.Contains(t, err.Error(), "participants must not be nil")
	})

	t.Run("invalid assignments", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			FirstView:          10,
			DKGPhase1FinalView: 20,
			DKGPhase2FinalView: 30,
			DKGPhase3FinalView: 40,
			FinalView:          50,
			Participants:       validParticipants,
			Assignments:        nil,
			RandomSource:       validRandomSource,
		}
		setup, err := flow.NewEpochSetup(untrusted)
		require.Error(t, err)
		require.Nil(t, setup)
		require.Contains(t, err.Error(), "assignments must not be nil")
	})

	t.Run("invalid RandomSource", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			FirstView:          10,
			DKGPhase1FinalView: 20,
			DKGPhase2FinalView: 30,
			DKGPhase3FinalView: 40,
			FinalView:          50,
			Participants:       validParticipants,
			Assignments:        validAssignments,
			RandomSource:       make([]byte, flow.EpochSetupRandomSourceLength-1), // too short
		}
		setup, err := flow.NewEpochSetup(untrusted)
		require.Error(t, err)
		require.Nil(t, setup)
		require.Contains(t, err.Error(), "random source must be of")
	})
	t.Run("invalid TargetDuration", func(t *testing.T) {
		untrusted := flow.UntrustedEpochSetup{
			FirstView:          10,
			DKGPhase1FinalView: 20,
			DKGPhase2FinalView: 30,
			DKGPhase3FinalView: 40,
			FinalView:          50,
			Participants:       validParticipants,
			Assignments:        validAssignments,
			RandomSource:       validRandomSource,
			TargetDuration:     0,
		}
		setup, err := flow.NewEpochSetup(untrusted)
		require.Error(t, err)
		require.Nil(t, setup)
		require.Contains(t, err.Error(), "target duration must be greater than 0")
	})
}

// TestNewEpochCommit validates the behavior of the NewEpochCommit constructor function.
// It checks for correct handling of both valid and invalid inputs.
//
// Test Cases:
//
// 1. Valid input returns commit:
//   - Ensures that providing all required and correctly formatted fields results in a successful creation of an EpochCommit instance.
//
// 2. Nil DKGGroupKey:
//   - Verifies that an error is returned when DKGGroupKey is nil.

// 3. Empty cluster QCs list:
//   - Verifies that an error is returned when cluster QCs list is empty.
//
// 4. Mismatched DKGParticipantKeys and DKGIndexMap lengths:
//   - Checks that an error is returned when the number of DKGParticipantKeys does not match the length of DKGIndexMap.
//
// 5. DKGIndexMap with out-of-range index:
//   - Ensures that an error is returned when DKGIndexMap contains an index outside the valid range.
//
// 6. DKGIndexMap with duplicate indices:
//   - Validates that an error is returned when DKGIndexMap contains duplicate indices.
func TestNewEpochCommit(t *testing.T) {
	// Setup common valid data
	validParticipantKeys := unittest.PublicKeysFixture(2, crypto.BLSBLS12381)
	validDKGGroupKey := unittest.KeyFixture(crypto.BLSBLS12381).PublicKey()
	validIndexMap := flow.DKGIndexMap{
		unittest.IdentifierFixture(): 0,
		unittest.IdentifierFixture(): 1,
	}
	validClusterQCs := []flow.ClusterQCVoteData{
		{
			VoterIDs: []flow.Identifier{
				unittest.IdentifierFixture(),
				unittest.IdentifierFixture(),
			},
			SigData: []byte{1, 1, 1},
		},
		{
			VoterIDs: []flow.Identifier{
				unittest.IdentifierFixture(),
				unittest.IdentifierFixture(),
			},
			SigData: []byte{2, 2, 2},
		},
	}

	t.Run("valid input", func(t *testing.T) {
		untrusted := flow.UntrustedEpochCommit{
			Counter:            1,
			ClusterQCs:         validClusterQCs,
			DKGGroupKey:        validDKGGroupKey,
			DKGParticipantKeys: validParticipantKeys,
			DKGIndexMap:        validIndexMap,
		}

		commit, err := flow.NewEpochCommit(untrusted)
		require.NoError(t, err)
		require.NotNil(t, commit)
	})

	t.Run("nil DKGGroupKey", func(t *testing.T) {
		untrusted := flow.UntrustedEpochCommit{
			Counter:            1,
			ClusterQCs:         validClusterQCs,
			DKGGroupKey:        nil,
			DKGParticipantKeys: validParticipantKeys,
			DKGIndexMap:        validIndexMap,
		}

		commit, err := flow.NewEpochCommit(untrusted)
		require.Error(t, err)
		require.Nil(t, commit)
		require.Contains(t, err.Error(), "DKG group key must not be nil")
	})

	t.Run("empty list of cluster QCs", func(t *testing.T) {
		untrusted := flow.UntrustedEpochCommit{
			Counter:            1,
			ClusterQCs:         []flow.ClusterQCVoteData{},
			DKGGroupKey:        validDKGGroupKey,
			DKGParticipantKeys: validParticipantKeys,
			DKGIndexMap:        validIndexMap,
		}

		commit, err := flow.NewEpochCommit(untrusted)
		require.Error(t, err)
		require.Nil(t, commit)
		require.Contains(t, err.Error(), "cluster QCs list must not be empty")
	})

	t.Run("mismatched DKGParticipantKeys and DKGIndexMap lengths", func(t *testing.T) {
		untrusted := flow.UntrustedEpochCommit{
			Counter:            1,
			ClusterQCs:         validClusterQCs,
			DKGGroupKey:        validDKGGroupKey,
			DKGParticipantKeys: unittest.PublicKeysFixture(1, crypto.BLSBLS12381), // Only one key
			DKGIndexMap:        validIndexMap,                                     // Two entries
		}

		commit, err := flow.NewEpochCommit(untrusted)
		require.Error(t, err)
		require.Nil(t, commit)
		require.Contains(t, err.Error(), "number of 1 Random Beacon key shares is inconsistent with number of DKG participants (len=2)")
	})

	t.Run("DKGIndexMap with out-of-range index", func(t *testing.T) {
		invalidIndexMap := flow.DKGIndexMap{
			unittest.IdentifierFixture(): 0,
			unittest.IdentifierFixture(): 2, // Index out of range for 2 participants
		}

		untrusted := flow.UntrustedEpochCommit{
			Counter:            1,
			ClusterQCs:         validClusterQCs,
			DKGGroupKey:        validDKGGroupKey,
			DKGParticipantKeys: validParticipantKeys,
			DKGIndexMap:        invalidIndexMap,
		}

		commit, err := flow.NewEpochCommit(untrusted)
		require.Error(t, err)
		require.Nil(t, commit)
		require.Contains(t, err.Error(), "index 2 is outside allowed range [0,n-1] for a DKG committee of size n=2")
	})

	t.Run("DKGIndexMap with duplicate indices", func(t *testing.T) {
		duplicateIndexMap := flow.DKGIndexMap{
			unittest.IdentifierFixture(): 0,
			unittest.IdentifierFixture(): 0, // Duplicate index
		}

		untrusted := flow.UntrustedEpochCommit{
			Counter:            1,
			ClusterQCs:         validClusterQCs,
			DKGGroupKey:        validDKGGroupKey,
			DKGParticipantKeys: validParticipantKeys,
			DKGIndexMap:        duplicateIndexMap,
		}

		commit, err := flow.NewEpochCommit(untrusted)
		require.Error(t, err)
		require.Nil(t, commit)
		require.Contains(t, err.Error(), "duplicated DKG index 0")
	})
}

// TestNewEpochRecover validates the behavior of the NewEpochRecover constructor function.
// It checks for correct handling of both valid and invalid inputs.
//
// Test Cases:
//
// 1. Valid input returns recover:
//   - Ensures that providing non-empty EpochSetup and EpochCommit results in a successful creation of an EpochRecover instance.
//
// 2. Empty EpochSetup:
//   - Verifies that an error is returned when EpochSetup is empty.
//
// 3. Empty EpochCommit:
//   - Checks that an error is returned when EpochCommit is empty.
//
// 4. Mismatched cluster counts:
//    - Validates that an error is returned when the number of Assignments in EpochSetup does not match the number of ClusterQCs in EpochCommit.
//
// 5. Mismatched epoch counters:
//    - Ensures that an error is returned when the Counter values in EpochSetup and EpochCommit do not match.

func TestNewEpochRecover(t *testing.T) {
	// Setup common valid data
	setupParticipants := unittest.IdentityListFixture(5, unittest.WithAllRoles()).Sort(flow.Canonical[flow.Identity])

	validSetup := unittest.EpochSetupFixture(
		unittest.SetupWithCounter(1),
		unittest.WithParticipants(setupParticipants.ToSkeleton()),
	)
	validCommit := unittest.EpochCommitFixture(
		unittest.CommitWithCounter(1),
		unittest.WithDKGFromParticipants(validSetup.Participants),
	)

	t.Run("valid input", func(t *testing.T) {
		untrusted := flow.UntrustedEpochRecover{
			EpochSetup:  *validSetup,
			EpochCommit: *validCommit,
		}

		recoverEpoch, err := flow.NewEpochRecover(untrusted)
		require.NoError(t, err)
		require.NotNil(t, recoverEpoch)
	})

	t.Run("empty EpochSetup", func(t *testing.T) {
		untrusted := flow.UntrustedEpochRecover{
			EpochSetup:  *new(flow.EpochSetup), // Empty setup
			EpochCommit: *validCommit,
		}

		recoverEpoch, err := flow.NewEpochRecover(untrusted)
		require.Error(t, err)
		require.Nil(t, recoverEpoch)
		require.Contains(t, err.Error(), "EpochSetup is empty")
	})

	t.Run("empty EpochCommit", func(t *testing.T) {
		untrusted := flow.UntrustedEpochRecover{
			EpochSetup:  *validSetup,
			EpochCommit: *new(flow.EpochCommit), // Empty commit
		}

		recoverEpoch, err := flow.NewEpochRecover(untrusted)
		require.Error(t, err)
		require.Nil(t, recoverEpoch)
		require.Contains(t, err.Error(), "EpochCommit is empty")
	})

	t.Run("mismatched cluster counts", func(t *testing.T) {
		// Create a copy of validSetup with an extra assignment
		mismatchedSetup := *validSetup
		mismatchedSetup.Assignments = unittest.ClusterAssignment(2, setupParticipants.ToSkeleton())

		untrusted := flow.UntrustedEpochRecover{
			EpochSetup:  mismatchedSetup,
			EpochCommit: *validCommit,
		}

		recoverEpoch, err := flow.NewEpochRecover(untrusted)
		require.Error(t, err)
		require.Nil(t, recoverEpoch)
		require.Contains(t, err.Error(), "does not match number of QCs")
	})

	t.Run("mismatched epoch counters", func(t *testing.T) {
		// Create a copy of validCommit with a different counter
		mismatchedCommit := *validCommit
		mismatchedCommit.Counter = validSetup.Counter + 1

		untrusted := flow.UntrustedEpochRecover{
			EpochSetup:  *validSetup,
			EpochCommit: mismatchedCommit,
		}

		recoverEpoch, err := flow.NewEpochRecover(untrusted)
		require.Error(t, err)
		require.Nil(t, recoverEpoch)
		require.Contains(t, err.Error(), "inconsistent epoch counter")
	})
}
