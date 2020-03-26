package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestHexStringToIdentifier(t *testing.T) {
	type testcase struct {
		hex         string
		expectError bool
	}

	cases := []testcase{{
		// non-hex characters
		hex:         "123456789012345678901234567890123456789012345678901234567890123z",
		expectError: true,
	}, {
		// too short
		hex:         "1234",
		expectError: true,
	}, {
		// just right
		hex:         "1234567890123456789012345678901234567890123456789012345678901234",
		expectError: false,
	}}

	for _, tcase := range cases {
		id, err := flow.HexStringToIdentifier(tcase.hex)
		if tcase.expectError {
			assert.Error(t, err)
			continue
		} else {
			assert.NoError(t, err)
		}

		assert.Equal(t, tcase.hex, id.String())
	}
}

func TestJSONEncodeDecode(t *testing.T) {
	id := unittest.IdentityFixture()
	enc, err := json.Marshal(id)
	require.NoError(t, err)
	var dec flow.Identity
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, *id, dec)
}

func TestJSONEncodeDecodeWithPublicKeys(t *testing.T) {
	id := unittest.IdentityFixture(unittest.WithRandomPublicKeys())
	enc, err := json.Marshal(id)
	require.NoError(t, err)
	var dec flow.Identity
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, *id, dec)
}
