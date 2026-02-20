package common

import (
	"fmt"
	"reflect"
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
		var out any
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

func TestConvertInterfaceToArrayOfStrings(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		expect    []string
		expectErr bool
	}{
		{
			name:      "Valid slice of strings",
			input:     []string{"a", "b", "c"},
			expect:    []string{"a", "b", "c"},
			expectErr: false,
		},
		{
			name:      "Valid slice of interfaces containing strings",
			input:     []any{"a", "b", "c"},
			expect:    []string{"a", "b", "c"},
			expectErr: false,
		},
		{
			name:      "Empty slice",
			input:     []any{},
			expect:    []string{},
			expectErr: false,
		},
		{
			name:      "Array contains nil value",
			input:     []any{"a", nil, "c"},
			expect:    nil,
			expectErr: true,
		},
		{
			name:      "Mixed types in slice",
			input:     []any{"a", 123, "c"},
			expect:    nil,
			expectErr: true,
		},
		{
			name:      "Non-array input",
			input:     42,
			expect:    nil,
			expectErr: true,
		},
		{
			name:      "Nil input",
			input:     nil,
			expect:    nil,
			expectErr: true,
		},
		{
			name:      "Slice with non-string interface values",
			input:     []any{true, false},
			expect:    nil,
			expectErr: true,
		},
		{
			name:      "Slice with nested slices",
			input:     []any{[]string{"a"}},
			expect:    nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertInterfaceToArrayOfStrings(tt.input)
			if (err != nil) != tt.expectErr {
				t.Fatalf("unexpected error status. got: %v, want error: %v", err, tt.expectErr)
			}
			if !reflect.DeepEqual(result, tt.expect) {
				t.Fatalf("unexpected result. got: %v, want: %v", result, tt.expect)
			}
		})
	}
}
