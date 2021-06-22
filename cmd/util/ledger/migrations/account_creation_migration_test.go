package migrations_test

import (
	"testing"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

func TestAccountCreationMigration(t *testing.T) {

	address := flow.HexToAddress("e467b9dd11fa00df")
	l := migrations.NewView(make([]ledger.Payload, 0))
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	err = accounts.SetContract("FlowServiceAccount", address, []byte("OLD CONTENT"))
	require.NoError(t, err)

	newPayloads, err := migrations.AccountCreationMigration(l.Payloads())
	require.NoError(t, err)

	st = state.NewState(migrations.NewView(newPayloads))
	sth = state.NewStateHolder(st)
	accounts = state.NewAccounts(sth)
	content, err := accounts.GetContract("FlowServiceAccount", address)
	require.NoError(t, err)
	require.Equal(t, content, migrations.AccountCreationContractContent())
}
