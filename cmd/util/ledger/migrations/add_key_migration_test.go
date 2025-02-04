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
}
