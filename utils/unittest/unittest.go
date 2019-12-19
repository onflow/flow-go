package unittest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/stretchr/testify/assert"
)

func ExpectPanic(expectedMsg string, t *testing.T) {
	if r := recover(); r != nil {
		err := r.(error)
		if err.Error() != expectedMsg {
			t.Errorf("expected %v to be %v", err, expectedMsg)
		}
		return
	}
	t.Errorf("Expected to panic with `%s`, but did not panic", expectedMsg)
}

// AssertEqualWithDiff asserts that two objects are equal.
//
// If the objects are not equal, this function prints a human-readable diff.
func AssertEqualWithDiff(t *testing.T, expected, actual interface{}) {
	// the maximum levels of a struct to recurse into
	// this prevents infinite recursion from circular references
	deep.MaxDepth = 100

	diff := deep.Equal(expected, actual)

	if len(diff) != 0 {
		s := strings.Builder{}

		for i, d := range diff {
			if i == 0 {
				s.WriteString("diff    : ")
			} else {
				s.WriteString("          ")
			}

			s.WriteString(d)
			s.WriteString("\n")
		}

		assert.Fail(t, fmt.Sprintf("Not equal: \n"+
			"expected: %s\n"+
			"actual  : %s\n\n"+
			"%s", expected, actual, s.String()))
	}
}
