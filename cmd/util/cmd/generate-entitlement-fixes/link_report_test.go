package generate_entitlement_fixes

import (
	"strings"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"
)

func TestReadLinkReport(t *testing.T) {
	t.Parallel()

	contents := `
      [
        {"address":"0x1","identifier":"foo","linkType":"&Foo","accessibleMembers":["foo"]},
        {"address":"0x2","identifier":"bar","linkType":"&Bar","accessibleMembers":null}
      ]
    `

	t.Run("unfiltered", func(t *testing.T) {

		t.Parallel()

		reader := strings.NewReader(contents)

		mapping, err := ReadPublicLinkReport(reader, nil)
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
	})

	t.Run("unfiltered", func(t *testing.T) {

		t.Parallel()

		address1 := common.MustBytesToAddress([]byte{0x1})

		reader := strings.NewReader(contents)

		mapping, err := ReadPublicLinkReport(
			reader,
			map[common.Address]struct{}{
				address1: {},
			})
		require.NoError(t, err)

		require.Equal(t,
			PublicLinkReport{
				{
					Address:    address1,
					Identifier: "foo",
				}: {
					BorrowType:        "&Foo",
					AccessibleMembers: []string{"foo"},
				},
			},
			mapping,
		)
	})
}
