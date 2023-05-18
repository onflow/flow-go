package cruisectl

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// TestParseTransition_Valid tests that valid transition configurations have
// consistent parsing and formatting behaviour.
func TestParseTransition_Valid(t *testing.T) {
	cases := []struct {
		transition EpochTransitionTime
		str        string
	}{{
		transition: EpochTransitionTime{time.Sunday, 0, 0},
		str:        "sunday@00:00",
	}, {
		transition: EpochTransitionTime{time.Wednesday, 8, 1},
		str:        "wednesday@08:01",
	}, {
		transition: EpochTransitionTime{time.Monday, 23, 59},
		str:        "monday@23:59",
	}, {
		transition: EpochTransitionTime{time.Friday, 12, 21},
		str:        "FrIdAy@12:21",
	}}

	for _, c := range cases {
		t.Run(c.str, func(t *testing.T) {
			// 1 - the computed string representation should match the string fixture
			assert.Equal(t, strings.ToLower(c.str), c.transition.String())
			// 2 - the parsed transition should match the transition fixture
			parsed, err := ParseTransition(c.str)
			assert.NoError(t, err)
			assert.Equal(t, c.transition, *parsed)
		})
	}
}

// TestParseTransition_Invalid tests that a selection of invalid transition strings
// fail validation and return an error.
func TestParseTransition_Invalid(t *testing.T) {
	cases := []string{
		// invalid WD part
		"sundy@12:00",
		"tue@12:00",
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

	for _, transitionStr := range cases {
		t.Run(transitionStr, func(t *testing.T) {
			_, err := ParseTransition(transitionStr)
			assert.Error(t, err)
		})
	}
}

func drawTransitionTime(t *rapid.T) EpochTransitionTime {
	day := time.Weekday(rapid.IntRange(0, 6).Draw(t, "wd").(int))
	hour := rapid.Uint8Range(0, 23).Draw(t, "h").(uint8)
	minute := rapid.Uint8Range(0, 59).Draw(t, "m").(uint8)
	return EpochTransitionTime{day, hour, minute}
}

func TestInferTargetEndTime(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		//ett := drawTransitionTime(t)

	})
}

// TestFindNearestTargetTime tests finding the nearest target time to a reference time.
func TestFindNearestTargetTime(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ett := drawTransitionTime(t)
		ref := time.Unix(rapid.Int64().Draw(t, "ref_unix").(int64), 0)

		nearest := ett.findNearestTargetTime(ref)
		distance := nearest.Sub(ref)
		// nearest date must be at most 4 days away
		// since distance is determined in terms of date, distance may be up to 4 days in time terms
		assert.Less(t, distance.Abs().Hours(), float64(24*4))
		// nearest date must be a target time
		assert.Equal(t, ett.day, nearest.Weekday())
		assert.Equal(t, int(ett.hour), nearest.Hour())
		assert.Equal(t, int(ett.minute), nearest.Minute())
	})
}
