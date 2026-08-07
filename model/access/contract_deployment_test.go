package access_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

func TestContractID(t *testing.T) {
	addr := flow.HexToAddress("0000000000000001")
	id := access.ContractID(addr, "Foo")
	assert.Equal(t, "A.0000000000000001.Foo", id)
}

func TestParseContractID(t *testing.T) {
	addr := flow.HexToAddress("0000000000000001")

	t.Run("valid ID", func(t *testing.T) {
		gotAddr, gotName, err := access.ParseContractID("A.0000000000000001.Foo")
		require.NoError(t, err)
		assert.Equal(t, addr, gotAddr)
		assert.Equal(t, "Foo", gotName)
	})

	t.Run("name with dots is invalid", func(t *testing.T) {
		_, _, err := access.ParseContractID("A.0000000000000001.Foo.Bar")
		require.Error(t, err)
	})

	t.Run("roundtrip with ContractID", func(t *testing.T) {
		id := access.ContractID(addr, "MyContract")
		gotAddr, gotName, err := access.ParseContractID(id)
		require.NoError(t, err)
		assert.Equal(t, addr, gotAddr)
		assert.Equal(t, "MyContract", gotName)
	})

	t.Run("empty string", func(t *testing.T) {
		_, _, err := access.ParseContractID("")
		require.Error(t, err)
	})

	t.Run("too short", func(t *testing.T) {
		_, _, err := access.ParseContractID("A.0000000000000001") // no name segment
		require.Error(t, err)
	})

	t.Run("wrong prefix", func(t *testing.T) {
		_, _, err := access.ParseContractID("B.0000000000000001.Foo")
		require.Error(t, err)
	})

	t.Run("no second dot", func(t *testing.T) {
		// "A." + 18 chars (no second dot) = 20 chars, passes length check
		_, _, err := access.ParseContractID("A.0000000000000001x")
		require.Error(t, err)
	})

	t.Run("invalid hex address", func(t *testing.T) {
		_, _, err := access.ParseContractID("A.zzzzzzzzzzzzzzzz.Foo")
		require.Error(t, err)
	})
}
