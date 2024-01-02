package request

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/access/rest/util"
)

const validBody = "access(all) fun main() { }"

var validBodyEncoded = util.ToBase64([]byte(validBody))

func TestScript_InvalidParse(t *testing.T) {
	test := map[string]string{
		"":                                     "request body must not be empty",
		"foo":                                  "request body contains badly-formed JSON (at position 2)",
		`{ "script": "123", "arguments": [] }`: "invalid script source encoding",
		fmt.Sprintf(`{ "script": "%s", "arguments": [123] }`, validBodyEncoded): `request body contains an invalid value for the "arguments" field (at position 69)`,
	}

	for in, errOut := range test {
		body := strings.NewReader(in)
		var script Script
		err := script.Parse(body)
		assert.EqualError(t, err, errOut, in)
	}
}

func TestScript_ValidParse(t *testing.T) {
	arg1 := []byte(`{"type": "String", "value": "hello" }`)
	body := strings.NewReader(fmt.Sprintf(
		`{ "script": "%s", "arguments": ["%s"] }`,
		validBodyEncoded,
		util.ToBase64(arg1),
	))

	var script Script
	err := script.Parse(body)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(script.Args))
	assert.Equal(t, arg1, script.Args[0])
	assert.Equal(t, validBody, string(script.Source))
}
