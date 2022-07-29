package unittest

import (
	"fmt"
	"testing"
)
import "os"

// TestCrashTest_ErrorMessage tests that CrashTest() can check a function that crashed without any messages.
func TestCrashTest_NoMessage(t *testing.T) {
	f := func(t *testing.T) {
		Crash_NoMessage(t)
	}

	CrashTest(t, f, "", "TestCrashTest_NoMessage")
}

// TestCrashTest_ErrorMessage tests that CrashTest() can read standard messages from stdout before a crash.
func TestCrashTest_ErrorMessage(t *testing.T) {
	f := func(t *testing.T) {
		Crash_ErrorMessage(t)
	}
	CrashTest(t, f, "about to crash", "TestCrashTest_ErrorMessage")
}

// TestCrashTest_Logger tests that CrashTest() can read fatal logger messages from stdout before a crash. This test
// assumes that the logger uses a hook to send fatal messages to stdout.
func TestCrashTest_Logger(t *testing.T) {
	f := func(t *testing.T) {
		Crash_LoggerFatal(t)
	}
	CrashTest(t, f, "fatal crash from logger", "TestCrashTest_Logger")
}

func Crash_NoMessage(t *testing.T) {
	os.Exit(1)
}

func Crash_ErrorMessage(t *testing.T) {
	fmt.Println("about to crash... crashing in 3...2...1...")
	os.Exit(1)
}

func Crash_LoggerFatal(t *testing.T) {
	// hook sends fatal messages to stdout, so they can be checked by CrashTest()
	logger, _ := HookedLogger()

	// calling Fatal() causes the process to exit
	logger.Fatal().Msg("fatal crash from logger... crashing in 3...2...1...")
}
