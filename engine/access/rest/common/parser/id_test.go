package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestID_InvalidParse(t *testing.T) {
	tests := map[string]string{
		"0x1": "invalid ID format",
		"foo": "invalid ID format",
	}

	for in, outErr := range tests {
		_, err := NewID(in)
		assert.EqualError(t, err, outErr)
	}
}

func TestID_ValidParse(t *testing.T) {
	testID := unittest.IdentifierFixture()

	id, err := NewID(testID.String())
	assert.NoError(t, err)
	assert.Equal(t, id.Flow().String(), testID.String())

	id, err = NewID("")
	assert.NoError(t, err)
	assert.Equal(t, id.Flow().String(), flow.ZeroID.String())
}

func TestIDs_ValidParse(t *testing.T) {
	testIDs := unittest.IdentifierListFixture(3)

	rawIDs := make([]string, 0)
	for _, id := range testIDs {
		rawIDs = append(rawIDs, id.String())
	}

	ids, err := NewIDs(rawIDs)
	assert.NoError(t, err)
	assert.Equal(t, testIDs, ids.Flow())

	// duplication of ids
	id := unittest.IdentifierFixture()
	duplicatedIDs := flow.IdentifierList{id, id, id}
	rawIDs = make([]string, 0)
	for _, id := range duplicatedIDs {
		rawIDs = append(rawIDs, id.String())
	}

	ids, err = NewIDs(rawIDs)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ids.Flow()))
	assert.Equal(t, ids.Flow()[0], id)
}

func TestIDs_InvalidParse(t *testing.T) {
	testIDs := make([]string, MaxIDsLength+1)
	_, err := NewIDs(testIDs)
	assert.EqualError(t, err, "at most 50 IDs can be requested at a time")
}
