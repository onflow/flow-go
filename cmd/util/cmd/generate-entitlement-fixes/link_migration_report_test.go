package generate_entitlement_fixes

import (
	"strings"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"
)

func TestReadPublicLinkMigrationReport(t *testing.T) {
	t.Parallel()

	contents := `
      [
        {"kind":"link-migration-success","account_address":"0x1","path":"/public/foo","capability_id":1},
        {"kind":"link-migration-success","account_address":"0x2","path":"/private/bar","capability_id":2},
        {"kind":"link-migration-success","account_address":"0x3","path":"/public/baz","capability_id":3}
      ]
    `

	t.Run("unfiltered", func(t *testing.T) {
		t.Parallel()

		reader := strings.NewReader(contents)

		mapping, err := ReadPublicLinkMigrationReport(reader, nil)
		require.NoError(t, err)

		require.Equal(t,
			PublicLinkMigrationReport{
				{
					Address:      common.MustBytesToAddress([]byte{0x1}),
					CapabilityID: 1,
				}: "foo",
				{
					Address:      common.MustBytesToAddress([]byte{0x3}),
					CapabilityID: 3,
				}: "baz",
			},
			mapping,
		)
	})

	t.Run("filtered", func(t *testing.T) {
		t.Parallel()

		address1 := common.MustBytesToAddress([]byte{0x1})

		reader := strings.NewReader(contents)

		mapping, err := ReadPublicLinkMigrationReport(
			reader,
			map[common.Address]struct{}{
				address1: {},
			},
		)
		require.NoError(t, err)

		require.Equal(t,
			PublicLinkMigrationReport{
				{
					Address:      address1,
					CapabilityID: 1,
				}: "foo",
			},
			mapping,
		)
	})
}
