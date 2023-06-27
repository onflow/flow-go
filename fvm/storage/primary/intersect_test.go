package primary

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestIntersect(t *testing.T) {
	check := func(
		writeSet map[flow.RegisterID]flow.RegisterValue,
		readSet map[flow.RegisterID]struct{},
		expectedMatch bool,
		expectedRegisterId flow.RegisterID) {

		match, registerId := intersectHelper(writeSet, readSet)
		require.Equal(t, match, expectedMatch)
		if match {
			require.Equal(t, expectedRegisterId, registerId)
		}

		match, registerId = intersectHelper(readSet, writeSet)
		require.Equal(t, match, expectedMatch)
		if match {
			require.Equal(t, expectedRegisterId, registerId)
		}

		match, registerId = intersect(writeSet, readSet)
		require.Equal(t, match, expectedMatch)
		if match {
			require.Equal(t, expectedRegisterId, registerId)
		}

		match, registerId = intersect(readSet, writeSet)
		require.Equal(t, match, expectedMatch)
		if match {
			require.Equal(t, expectedRegisterId, registerId)
		}
	}

	owner := "owner"
	key1 := "key1"
	key2 := "key2"

	// set up readSet1 and writeSet1 such that len(readSet1) > len(writeSet1),
	// and shares key1

	readSet1 := map[flow.RegisterID]struct{}{
		flow.RegisterID{
			Owner: owner,
			Key:   key1,
		}: struct{}{},
		flow.RegisterID{
			Owner: "1",
			Key:   "read 1",
		}: struct{}{},
		flow.RegisterID{
			Owner: "1",
			Key:   "read 2",
		}: struct{}{},
	}

	writeSet1 := map[flow.RegisterID]flow.RegisterValue{
		flow.RegisterID{
			Owner: owner,
			Key:   key1,
		}: []byte("blah"),
		flow.RegisterID{
			Owner: "1",
			Key:   "write",
		}: []byte("blah"),
	}

	// set up readSet2 and writeSet2 such that len(readSet2) < len(writeSet2),
	// shares key2, and not share keys with readSet1 / writeSet1

	readSet2 := map[flow.RegisterID]struct{}{
		flow.RegisterID{
			Owner: owner,
			Key:   key2,
		}: struct{}{},
	}

	writeSet2 := map[flow.RegisterID]flow.RegisterValue{
		flow.RegisterID{
			Owner: owner,
			Key:   key2,
		}: []byte("blah"),
		flow.RegisterID{
			Owner: "2",
			Key:   "write 1",
		}: []byte("blah"),
		flow.RegisterID{
			Owner: "2",
			Key:   "write 2",
		}: []byte("blah"),
		flow.RegisterID{
			Owner: "2",
			Key:   "write 3",
		}: []byte("blah"),
	}

	check(writeSet1, readSet1, true, flow.RegisterID{Owner: owner, Key: key1})
	check(writeSet2, readSet2, true, flow.RegisterID{Owner: owner, Key: key2})

	check(writeSet1, readSet2, false, flow.RegisterID{})
	check(writeSet2, readSet1, false, flow.RegisterID{})
}
