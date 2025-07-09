package request

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/assert"
)

func Test_GetByID_Parse(t *testing.T) {
	var getByID GetByIDRequest

	id := unittest.IdentifierFixture()
	err := getByID.Parse(id.String())
	assert.NoError(t, err)
	assert.Equal(t, getByID.ID, id)

	err = getByID.Parse("1")
	assert.EqualError(t, err, "invalid ID format")
}
