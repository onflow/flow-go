package unittest

import "testing"
import "os"

func TestCrashTest(t *testing.T) {
	f := func() {
		CrashAndBurn(t)
	}

	CrashTest(t, f)
}

func CrashAndBurn(t *testing.T) {
	os.Exit(1)
}
