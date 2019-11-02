package flow_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/magiconair/properties/assert"

	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

type addressWrapper struct {
	Address flow.Address
}

func TestAddressJSON(t *testing.T) {
	addr := unittest.AddressFixture()
	data, err := json.Marshal(addressWrapper{Address: addr})
	require.Nil(t, err)
	fmt.Println(addr.Hex())

	t.Log(string(data))

	var out addressWrapper
	err = json.Unmarshal(data, &out)
	require.Nil(t, err)
	assert.Equal(t, addr, out.Address)
}
