package common

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		err := ParseBody(readerIn, out)
		assert.EqualError(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}

	type body struct {
		Foo string
		Bar bool
		Zoo uint64
	}
	var b body
	err := ParseBody(strings.NewReader(`{ "foo": "test", "bar": true }`), &b)
	assert.NoError(t, err)
	assert.Equal(t, b.Bar, true)
	assert.Equal(t, b.Foo, "test")
	assert.Equal(t, b.Zoo, uint64(0))

	err = ParseBody(strings.NewReader(`{ "foo": false }`), &b)
	assert.EqualError(t, err, `request body contains an invalid value for the "Foo" field (at position 14)`)
}
