package badger

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func ProtocolState(t testing.TB, db *badger.DB, options ...func(*State)) *State {
	identities := storage.NewIdentities(db)
	guarantees := storage.NewGuarantees(db)
	seals := storage.NewSeals(db)
	headers := storage.NewHeaders(db)
	payloads := storage.NewPayloads(db, identities, guarantees, seals)
	blocks := storage.NewBlocks(db, headers, payloads)
	proto, err := NewState(db, blocks, headers, payloads, identities, seals, options...)
	require.NoError(t, err)
	return proto
}

func RunWithProtocolState(t testing.TB, f func(*badger.DB, *State), options ...func(*State)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		proto := ProtocolState(t, db, options...)
		f(db, proto)
	})
}
