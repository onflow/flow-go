package gossip

import (
	"fmt"
	"strings"
	"testing"
)

var (
	errs = []error{
		fmt.Errorf("first error"),
		fmt.Errorf("second error"),
		fmt.Errorf("third error"),
		fmt.Errorf("fourth error"),
		fmt.Errorf("fifth error"),
	}
)

func TestCollect(t *testing.T) {
	gerr := &gossipError{}

	for _, err := range errs {
		gerr.Append(err)
	}

	if len(*gerr) != len(errs) {
		t.Errorf("number of errors collected mismatch. expected: %v, got: %v", len(errs), len(*gerr))
	}

}

func TestError(t *testing.T) {
	gerr := gossipError(errs)

	errorString := gerr.Error()

	for _, err := range errs {
		if !strings.Contains(errorString, err.Error()) {
			t.Errorf("error string does not contain error: %v", err.Error())
		}

	}
}
