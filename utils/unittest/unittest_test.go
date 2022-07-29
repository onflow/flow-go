package unittest

import (
	"fmt"
	"testing"
)
import "os"

func TestCrashTest_NoMessage(t *testing.T) {
	f := func() {
		CrashAndBurn_NoMessage(t)
	}

	CrashTest(t, f, "")
}

func TestCrashTest_ErrorMessage(t *testing.T) {
	f := func() {
		CrashAndBurn_NoMessage(t)
	}

	CrashTest(t, f, "about to crash")

}

func CrashAndBurn_NoMessage(t *testing.T) {
	os.Exit(1)
}

func CrashAndBurn_ErrorMessage(t *testing.T) {
	t.Log("about to crash... crashing in 3...2...1...")
	fmt.Println("about to crash... crashing in 3...2...1...")
	os.Exit(1)
}
