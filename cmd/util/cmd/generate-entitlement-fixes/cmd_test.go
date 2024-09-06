package generate_entitlement_fixes

import (
	"strings"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"
)

func TestReadPublicLinkMigrationReport(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(`
      [
        {"kind":"link-migration-success","account_address":"0x1","path":"/public/foo","capability_id":1},
        {"kind":"link-migration-success","account_address":"0x2","path":"/private/bar","capability_id":2}
      ]
    `)

	mapping, err := ReadPublicLinkMigrationReport(reader)
	require.NoError(t, err)

	require.Equal(t,
		PublicLinkMigrationReport{
			{
				Address:      common.MustBytesToAddress([]byte{0x1}),
				CapabilityID: 1,
			}: "foo",
		},
		mapping,
	)
}

func TestReadLinkReport(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(`
      [
        {"address":"0x1","identifier":"foo","linkType":"&Foo","accessibleMembers":["foo"]},
        {"address":"0x2","identifier":"bar","linkType":"&Bar","accessibleMembers":null}
      ]
    `)

	mapping, err := ReadPublicLinkReport(reader)
	require.NoError(t, err)

	require.Equal(t,
		PublicLinkReport{
			{
				Address:    common.MustBytesToAddress([]byte{0x1}),
				Identifier: "foo",
			}: {
				BorrowType:        "&Foo",
				AccessibleMembers: []string{"foo"},
			},
			{
				Address:    common.MustBytesToAddress([]byte{0x2}),
				Identifier: "bar",
			}: {
				BorrowType:        "&Bar",
				AccessibleMembers: nil,
			},
		},
		mapping,
	)
}
