// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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

func TestUnknownDecode(t *testing.T) {
	// pass an encoded Identifier to `d`
	id := unittest.IdentifierFixture()
	fmt.Println(id)
	enc := b(id)
	dec := d(enc)
	fmt.Println(dec)
}
