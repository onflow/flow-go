package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
)

func TestAccountStatus(t *testing.T) {

	s := state.NewAccountStatus()
	require.True(t, s.AccountExists())
	require.False(t, s.IsAccountFrozen())

	s = state.SetAccountStatusFrozenFlag(s, true)
	require.True(t, s.AccountExists())
	require.True(t, s.IsAccountFrozen())

	s = state.SetAccountStatusFrozenFlag(s, false)
	require.True(t, s.AccountExists())
	require.False(t, s.IsAccountFrozen())

	var err error
	s, err = state.AccountStatusFromBytes(s.ToBytes())
	require.NoError(t, err)
	require.True(t, s.AccountExists())
	require.False(t, s.IsAccountFrozen())
}
