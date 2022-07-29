package unittest

import (
	"fmt"
	"testing"
)
import "os"

// TestCrashTest_ErrorMessage tests that CrashTest() can check a function that crashed without any messages.
func TestCrashTest_NoMessage(t *testing.T) {
	f := func(t *testing.T) {
		CrashAndBurn_NoMessage(t)
	}

	CrashTest(t, f, "", "TestCrashTest_NoMessage")
}

// TestCrashTest_ErrorMessage tests that CrashTest() can read standard messages from stdout before a crash.
func TestCrashTest_ErrorMessage(t *testing.T) {
	f := func(t *testing.T) {
		CrashAndBurn_ErrorMessage(t)
	}
	CrashTest(t, f, "about to crash", "TestCrashTest_ErrorMessage")
}

func CrashAndBurn_NoMessage(t *testing.T) {
	os.Exit(1)
}

func CrashAndBurn_ErrorMessage(t *testing.T) {
	fmt.Println("about to crash... crashing in 3...2...1...")
	os.Exit(1)
}
