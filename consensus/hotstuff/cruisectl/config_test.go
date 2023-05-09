package cruisectl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestParseSwitchover_Valid tests that valid switchover configurations have
// consistent parsing and formatting behaviour.
func TestParseSwitchover_Valid(t *testing.T) {
	cases := []struct {
		switchover Switchover
		str        string
	}{{
		switchover: Switchover{time.Sunday, 0, 0},
		str:        "sunday@00:00",
	}, {
		switchover: Switchover{time.Wednesday, 8, 1},
		str:        "wednesday@08:01",
	}, {
		switchover: Switchover{time.Friday, 23, 59},
		str:        "monday@23:59",
	}}

	for _, c := range cases {
		t.Run(c.str, func(t *testing.T) {
			// 1 - the computed string representation should match the string fixture
			assert.Equal(t, c.str, c.switchover.String())
			// 2 - the parsed switchover should match the switchover fixture
			parsed, err := ParseSwitchover(c.str)
			assert.NoError(t, err)
			assert.Equal(t, c.switchover, parsed)
		})
	}
}

// TestParseSwitchover_Invalid tests that a selection of invalid switchover strings
// fail validation and return an error.
func TestParseSwitchover_Invalid(t *testing.T) {
	cases := []string{
		// invalid WD part
		"sundy@12:00",
		"tue@12:00",
		"Monday@12:00",
		"@12:00",
		// invalid HH part
		"wednesday@24:00",
		"wednesday@1:00",
		"wednesday@:00",
		"wednesday@012:00",
		// invalid MM part
		"wednesday@12:60",
		"wednesday@12:1",
		"wednesday@12:",
		"wednesday@12:030",
		// otherwise invalid
		"",
		"@:",
		"monday@@12:00",
		"monday@09:00am",
		"monday@09:00PM",
		"monday12:00",
		"monday12::00",
		"wednesday@1200",
	}

	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := ParseSwitchover(c)
			assert.Error(t, err)
		})
	}
}
