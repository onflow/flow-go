package request

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestID_InvalidParse(t *testing.T) {
	tests := map[string]string{
		"0x1": "invalid ID format",
		"foo": "invalid ID format",
	}

	var id ID
	for in, outErr := range tests {
		err := id.Parse(in)
		assert.EqualError(t, err, outErr)
	}
}

func TestID_ValidParse(t *testing.T) {
	testID := unittest.IdentifierFixture()

	var id ID
	err := id.Parse(testID.String())
	assert.NoError(t, err)
	assert.Equal(t, id.Flow().String(), testID.String())

	err = id.Parse("")
	assert.NoError(t, err)
	assert.Equal(t, id.Flow().String(), flow.ZeroID.String())
}

func TestIDs_ValidParse(t *testing.T) {
	testIDs := unittest.IdentifierListFixture(3)

	var ids IDs
	rawIDs := make([]string, 0)
	for _, id := range testIDs {
		rawIDs = append(rawIDs, id.String())
	}

	err := ids.Parse(rawIDs)
	assert.NoError(t, err)
	assert.Equal(t, testIDs, ids.Flow())

	// duplication of ids
	id := unittest.IdentifierFixture()
	duplicatedIDs := []flow.Identifier{id, id, id}
	rawIDs = make([]string, 0)
	for _, id := range duplicatedIDs {
		rawIDs = append(rawIDs, id.String())
	}

	err = ids.Parse(rawIDs)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ids.Flow()))
	assert.Equal(t, ids.Flow()[0], id)
}

func TestIDs_InvalidParse(t *testing.T) {
	testIDs := make([]string, maxIDsLength+1)
	var ids IDs
	err := ids.Parse(testIDs)
	assert.EqualError(t, err, "at most 50 IDs can be requested at a time")
}
