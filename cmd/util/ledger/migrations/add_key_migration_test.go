package migrations

import (
	"testing"
)

func Test_fail_if_migration_enabled(t *testing.T) {
	t.Parallel()
	// prevent merging this to master branch if enabled
	if IAmSureIWantToRunThisMigration {
		t.Fail()
	}

	// a reminder to set this back to false so that we have
	// it prepared for next time.
	if IHaveCheckedTheMainnetNodeAddressesForCorrectness {
		t.Fail()
	}
}
