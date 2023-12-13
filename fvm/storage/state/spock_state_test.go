package state

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/rand"
	"github.com/onflow/flow-go/utils/unittest"
)

type spockTestOp func(*testing.T, *spockState)

var fooOwner = unittest.RandomAddressFixture()

func chainSpockTestOps(prevOps spockTestOp, op spockTestOp) spockTestOp {
	return func(t *testing.T, state *spockState) {
		if prevOps != nil {
			prevOps(t, state)
		}
		op(t, state)
	}
}

func testSpock(
	t *testing.T,
	counterfactualExperiments []spockTestOp,
) []*spockState {
	resultStates := []*spockState{}
	for _, experiment := range counterfactualExperiments {
		run1 := newSpockState(snapshot.MapStorageSnapshot{})
		run2 := newSpockState(snapshot.MapStorageSnapshot{})

		if experiment != nil {
			experiment(t, run1)
			experiment(t, run2)
		}

		spock := run1.Finalize().SpockSecret
		require.Equal(t, spock, run2.Finalize().SpockSecret)

		for _, previous := range resultStates {
			require.NotEqual(t, spock, previous.Finalize().SpockSecret)
		}

		resultStates = append(resultStates, run1)
	}

	return resultStates
}

func TestSpockStateGet(t *testing.T) {
	otherOwner := unittest.RandomAddressFixture()
	registerId := flow.NewRegisterID(fooOwner, "bar")

	states := testSpock(
		t,
		[]spockTestOp{
			// control experiment
			nil,
			// primary experiment
			func(t *testing.T, state *spockState) {
				_, err := state.Get(registerId)
				require.NoError(t, err)
			},
			// duplicate calls return in different spock
			func(t *testing.T, state *spockState) {
				_, err := state.Get(registerId)
				require.NoError(t, err)
				_, err = state.Get(registerId)
				require.NoError(t, err)
			},
			// Reading different register ids will result in different spock
			func(t *testing.T, state *spockState) {
				_, err := state.Get(flow.NewRegisterID(otherOwner, "bar"))
				require.NoError(t, err)
			},
			func(t *testing.T, state *spockState) {
				_, err := state.Get(flow.NewRegisterID(fooOwner, "baR"))
				require.NoError(t, err)
			},
		})

	// Sanity check underlying storage state is called.
	require.Equal(
		t,
		map[flow.RegisterID]struct{}{
			registerId: struct{}{},
		},
		states[1].Finalize().ReadSet)

	// Sanity check finalized state is no longer accessible.
	_, err := states[1].Get(registerId)
	require.ErrorContains(t, err, "cannot Get on a finalized state")
}

func TestSpockStateGetDifferentUnderlyingStorage(t *testing.T) {
	badRegisterId := flow.NewRegisterID(fooOwner, "bad")

	value1 := flow.RegisterValue([]byte("abc"))
	value2 := flow.RegisterValue([]byte("blah"))

	state1 := newSpockState(
		snapshot.MapStorageSnapshot{
			badRegisterId: value1,
		})

	state2 := newSpockState(
		snapshot.MapStorageSnapshot{
			badRegisterId: value2,
		})

	value, err := state1.Get(badRegisterId)
	require.NoError(t, err)
	require.Equal(t, value1, value)

	value, err = state2.Get(badRegisterId)
	require.NoError(t, err)
	require.Equal(t, value2, value)

	// state1 and state2 will have identical spock hash even through they read
	// different values from the underlying storage.  Merkle trie proof will
	// ensure the underlying storage is correct / identical.
	require.Equal(
		t,
		state1.Finalize().SpockSecret,
		state2.Finalize().SpockSecret)
}

func TestSpockStateGetVsSetNil(t *testing.T) {
	registerId := flow.NewRegisterID(fooOwner, "bar")

	_ = testSpock(
		t,
		[]spockTestOp{
			func(t *testing.T, state *spockState) {
				err := state.Set(registerId, []byte{})
				require.NoError(t, err)
			},
			func(t *testing.T, state *spockState) {
				_, err := state.Get(registerId)
				require.NoError(t, err)
			},
		})
}

func TestSpockStateSet(t *testing.T) {
	otherOwner := unittest.RandomAddressFixture()
	registerId := flow.NewRegisterID(fooOwner, "bar")
	value := flow.RegisterValue([]byte("value"))

	states := testSpock(
		t,
		[]spockTestOp{
			// control experiment
			nil,
			// primary experiment
			func(t *testing.T, state *spockState) {
				err := state.Set(registerId, value)
				require.NoError(t, err)
			},
			// duplicate calls return in different spock
			func(t *testing.T, state *spockState) {
				err := state.Set(registerId, value)
				require.NoError(t, err)
				err = state.Set(registerId, value)
				require.NoError(t, err)
			},
			// Setting different register id will result in different spock
			func(t *testing.T, state *spockState) {
				err := state.Set(flow.NewRegisterID(fooOwner, "baR"), value)
				require.NoError(t, err)
			},
			func(t *testing.T, state *spockState) {
				err := state.Set(flow.NewRegisterID(otherOwner, "bar"), value)
				require.NoError(t, err)
			},
			// Setting different register value will result in different spock
			func(t *testing.T, state *spockState) {
				err := state.Set(registerId, []byte("valuE"))
				require.NoError(t, err)
			},
		})

	// Sanity check underlying storage state is called.
	require.Equal(
		t,
		map[flow.RegisterID]flow.RegisterValue{
			registerId: value,
		},
		states[1].Finalize().WriteSet)

	// Sanity check finalized state is no longer accessible.
	err := states[1].Set(registerId, []byte(""))
	require.ErrorContains(t, err, "cannot Set on a finalized state")
}

func TestSpockStateSetValueInjection(t *testing.T) {
	registerId1 := flow.NewRegisterID(fooOwner, "injection")
	registerId2 := flow.NewRegisterID(fooOwner, "inject")

	_ = testSpock(
		t,
		[]spockTestOp{
			func(t *testing.T, state *spockState) {
				err := state.Set(registerId1, []byte{})
				require.NoError(t, err)
			},
			func(t *testing.T, state *spockState) {
				err := state.Set(registerId2, []byte("ion"))
				require.NoError(t, err)
			},
		})
}

func TestSpockStateMerge(t *testing.T) {
	readSet := map[flow.RegisterID]struct{}{
		flow.NewRegisterID(fooOwner, "bar"): struct{}{},
	}

	states := testSpock(
		t,
		[]spockTestOp{
			// control experiment
			nil,
			// primary experiment
			func(t *testing.T, state *spockState) {
				err := state.Merge(
					&snapshot.ExecutionSnapshot{
						ReadSet:     readSet,
						SpockSecret: []byte("secret"),
					})
				require.NoError(t, err)
			},
			// duplicate calls result in different spock
			func(t *testing.T, state *spockState) {
				err := state.Merge(
					&snapshot.ExecutionSnapshot{
						ReadSet:     readSet,
						SpockSecret: []byte("secret"),
					})
				require.NoError(t, err)
				err = state.Merge(
					&snapshot.ExecutionSnapshot{
						ReadSet:     readSet,
						SpockSecret: []byte("secret"),
					})
				require.NoError(t, err)
			},
			// Merging execution snapshot with different spock will result in
			// different spock
			func(t *testing.T, state *spockState) {
				err := state.Merge(
					&snapshot.ExecutionSnapshot{
						ReadSet:     readSet,
						SpockSecret: []byte("secreT"),
					})
				require.NoError(t, err)
			},
		})

	// Sanity check underlying storage state is called.
	require.Equal(t, readSet, states[1].Finalize().ReadSet)

	// Sanity check finalized state is no longer accessible.
	err := states[1].Merge(&snapshot.ExecutionSnapshot{})
	require.ErrorContains(t, err, "cannot Merge on a finalized state")
}
func TestSpockStateDropChanges(t *testing.T) {
	registerId := flow.NewRegisterID(fooOwner, "read")

	setup := func(t *testing.T, state *spockState) {
		_, err := state.Get(registerId)
		require.NoError(t, err)

		err = state.Set(flow.NewRegisterID(fooOwner, "write"), []byte("blah"))
		require.NoError(t, err)
	}

	states := testSpock(
		t,
		[]spockTestOp{
			// control experiment
			setup,
			// primary experiment
			func(t *testing.T, state *spockState) {
				setup(t, state)
				err := state.DropChanges()
				require.NoError(t, err)
			},
			// duplicate calls result in different spock
			func(t *testing.T, state *spockState) {
				setup(t, state)
				err := state.DropChanges()
				require.NoError(t, err)
				err = state.DropChanges()
				require.NoError(t, err)
			},
		})

	// Sanity check underlying storage state is called.
	snapshot := states[1].Finalize()
	require.Equal(
		t,
		map[flow.RegisterID]struct{}{
			registerId: struct{}{},
		},
		snapshot.ReadSet)
	require.Empty(t, snapshot.WriteSet)

	// Sanity check finalized state is no longer accessible.
	err := states[1].DropChanges()
	require.ErrorContains(t, err, "cannot DropChanges on a finalized state")
}

func TestSpockStateRandomOps(t *testing.T) {
	chain := []spockTestOp{
		nil, // control experiment
	}

	for i := 0; i < 500; i++ {
		roll, err := rand.Uintn(4)
		require.NoError(t, err)

		switch roll {
		case uint(0):
			id, err := rand.Uint()
			require.NoError(t, err)

			chain = append(
				chain,
				chainSpockTestOps(
					chain[len(chain)-1],
					func(t *testing.T, state *spockState) {
						_, err := state.Get(
							flow.NewRegisterID(flow.EmptyAddress, fmt.Sprintf("%d", id)))
						require.NoError(t, err)
					}))
		case uint(1):
			id, err := rand.Uint()
			require.NoError(t, err)

			value, err := rand.Uint()
			require.NoError(t, err)

			chain = append(
				chain,
				chainSpockTestOps(
					chain[len(chain)-1],
					func(t *testing.T, state *spockState) {
						err := state.Set(
							flow.NewRegisterID(flow.EmptyAddress, fmt.Sprintf("%d", id)),
							[]byte(fmt.Sprintf("%d", value)))
						require.NoError(t, err)
					}))
		case uint(2):
			spock, err := rand.Uint()
			require.NoError(t, err)

			chain = append(
				chain,
				chainSpockTestOps(
					chain[len(chain)-1],
					func(t *testing.T, state *spockState) {
						err := state.Merge(
							&snapshot.ExecutionSnapshot{
								SpockSecret: []byte(fmt.Sprintf("%d", spock)),
							})
						require.NoError(t, err)
					}))
		case uint(3):
			chain = append(
				chain,
				chainSpockTestOps(
					chain[len(chain)-1],
					func(t *testing.T, state *spockState) {
						err := state.DropChanges()
						require.NoError(t, err)
					}))
		default:
			panic("Unexpected")
		}
	}

	_ = testSpock(t, chain)
}
func TestSpockStateNewChild(t *testing.T) {
	baseRegisterId := flow.NewRegisterID(flow.EmptyAddress, "base")
	baseValue := flow.RegisterValue([]byte("base"))

	parentOwner := unittest.RandomAddressFixture()
	childOwner := unittest.RandomAddressFixture()

	parentRegisterId1 := flow.NewRegisterID(parentOwner, "1")
	parentValue := flow.RegisterValue([]byte("parent"))

	parentRegisterId2 := flow.NewRegisterID(parentOwner, "2")

	childRegisterId1 := flow.NewRegisterID(childOwner, "1")
	childValue := flow.RegisterValue([]byte("child"))

	childRegisterId2 := flow.NewRegisterID(childOwner, "2")

	parent := newSpockState(
		snapshot.MapStorageSnapshot{
			baseRegisterId: baseValue,
		})

	err := parent.Set(parentRegisterId1, parentValue)
	require.NoError(t, err)

	value, err := parent.Get(parentRegisterId2)
	require.NoError(t, err)
	require.Nil(t, value)

	child := parent.NewChild()

	value, err = child.Get(baseRegisterId)
	require.NoError(t, err)
	require.Equal(t, value, baseValue)

	value, err = child.Get(parentRegisterId1)
	require.NoError(t, err)
	require.Equal(t, value, parentValue)

	value, err = child.Get(childRegisterId2)
	require.NoError(t, err)
	require.Nil(t, value)

	err = child.Set(childRegisterId1, childValue)
	require.NoError(t, err)

	childSnapshot := child.Finalize()
	require.Equal(
		t,
		childSnapshot.ReadSet,
		map[flow.RegisterID]struct{}{
			baseRegisterId:    struct{}{},
			parentRegisterId1: struct{}{},
			childRegisterId2:  struct{}{},
		})

	require.Equal(
		t,
		childSnapshot.WriteSet,
		map[flow.RegisterID]flow.RegisterValue{
			childRegisterId1: childValue,
		})

	// Finalize parent without merging child to see if they are independent.
	parentSnapshot := parent.Finalize()
	require.Equal(
		t,
		parentSnapshot.ReadSet,
		map[flow.RegisterID]struct{}{
			parentRegisterId2: struct{}{},
		})

	require.Equal(
		t,
		parentSnapshot.WriteSet,
		map[flow.RegisterID]flow.RegisterValue{
			parentRegisterId1: parentValue,
		})
}
