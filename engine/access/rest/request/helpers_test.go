package request

import (
	"fmt"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
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

func Test_ParseBody(t *testing.T) {

	invalid := []struct {
		in  string
		err string
	}{
		{"{f}", "request body contains badly-formed JSON (at position 2)"},
		{"foo", "request body contains badly-formed JSON (at position 2)"},
		{"", "request body must not be empty"},
		{`{"foo": "bar"`, "request body contains badly-formed JSON"},
		{`{"foo": "bar" "foo2":"bar2"}`, "request body contains badly-formed JSON (at position 15)"},
		{`{"foo":"bar"}, {}`, "request body must only contain a single JSON object"},
		{`[][]`, "request body must only contain a single JSON object"},
	}

	for i, test := range invalid {
		readerIn := strings.NewReader(test.in)
		var out interface{}
		err := parseBody(readerIn, out)
		assert.EqualError(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}

	type body struct {
		Foo string
		Bar bool
		Zoo uint64
	}
	var b body
	err := parseBody(strings.NewReader(`{ "foo": "test", "bar": true }`), &b)
	assert.NoError(t, err)
	assert.Equal(t, b.Bar, true)
	assert.Equal(t, b.Foo, "test")
	assert.Equal(t, b.Zoo, uint64(0))

	err = parseBody(strings.NewReader(`{ "foo": false }`), &b)
	assert.EqualError(t, err, `request body contains an invalid value for the "Foo" field (at position 14)`)
}
