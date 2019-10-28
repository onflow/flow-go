package gnode

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mock errors for the purpose of testing
var (
	errs = []error{
		fmt.Errorf("first error"),
		fmt.Errorf("second error"),
		fmt.Errorf("third error"),
		fmt.Errorf("fourth error"),
		fmt.Errorf("fifth error"),
	}
)

//TestAppend tests if Append method is working properly using mock errors.
func TestAppend(t *testing.T) {
	gerr := &gossipError{}

	for _, err := range errs {
		gerr.Append(err)
	}

	// check if number of collected errors matches original errors array
	assert.Equal(t, len(*gerr), len(errs))
}

//TestError tests Error method
func TestError(t *testing.T) {
	gerr := gossipError(errs)

	errorString := gerr.Error()

	for _, err := range errs {
		if !strings.Contains(errorString, err.Error()) {
			t.Errorf("error string does not contain error: %v", err.Error())
		}

	}
}
