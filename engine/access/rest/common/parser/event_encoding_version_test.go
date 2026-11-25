package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

func TestParseEventEncodingVersion_Valid(t *testing.T) {
	cases := map[string]entities.EventEncodingVersion{
		"":            entities.EventEncodingVersion_JSON_CDC_V0,
		"JSON_CDC_V0": entities.EventEncodingVersion_JSON_CDC_V0,
		"json_cdc_v0": entities.EventEncodingVersion_JSON_CDC_V0,
		"JSON-CDC-V0": entities.EventEncodingVersion_JSON_CDC_V0,
		"json-cdc-v0": entities.EventEncodingVersion_JSON_CDC_V0,
		"CCF_V0":      entities.EventEncodingVersion_CCF_V0,
		"ccf_v0":      entities.EventEncodingVersion_CCF_V0,
		"CCF-V0":      entities.EventEncodingVersion_CCF_V0,
		"ccf-v0":      entities.EventEncodingVersion_CCF_V0,
	}

	for input, expected := range cases {
		got, err := ParseEventEncodingVersion(input)
		assert.NoError(t, err, "input %q", input)
		assert.Equal(t, expected, got, "input %q", input)
	}
}

func TestParseEventEncodingVersion_Invalid(t *testing.T) {
	for _, input := range []string{"json", "ccf", "unknown", "JSON_CDC_V1"} {
		_, err := ParseEventEncodingVersion(input)
		assert.Error(t, err, "input %q", input)
	}
}
