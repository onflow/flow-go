package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

func TestMakePrefix(t *testing.T) {

	code := byte(0x01)

	u := uint64(1337)
	expected := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05, 0x39}
	actual := makePrefix(code, u)

	assert.Equal(t, expected, actual)

	r := flow.Role(2)
	expected = []byte{0x01, 0x02}
	actual = makePrefix(code, r)

	assert.Equal(t, expected, actual)

	id := flow.Identifier{0x05, 0x06, 0x07}
	expected = []byte{0x01,
		0x05, 0x06, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	actual = makePrefix(code, id)

	assert.Equal(t, expected, actual)
}
