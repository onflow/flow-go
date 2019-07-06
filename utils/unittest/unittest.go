package unittest

import (
	"testing"
)

func ExpectNoError(err error, t *testing.T) {
	if err != nil {
		t.Error(err)
	}
}

func ExpectError(err error, t *testing.T) {
	if err == nil {
		t.Error("Expected error but got err=nil")
	}
}

func ExpectBool(actual, expected bool, t *testing.T) {
	if actual != expected {
		t.Errorf("expected %v to be %v", actual, expected)
	}
}

func ExpectInt(actual, expected int, t *testing.T) {
	if actual != expected {
		t.Errorf("expected %v to be %v", actual, expected)
	}
}

func ExpectString(actual, expected string, t *testing.T) {
	if actual != expected {
		t.Errorf("expected %v to be %v", actual, expected)
	}
}

func ExpectStrings(actual, expected []string, t *testing.T) {
	ExpectInt(len(actual), len(expected), t)
	for i := range actual {
		ExpectString(actual[i], expected[i], t)
	}
}

func ExpectPanic(expectedMsg string, t *testing.T) {
	if r := recover(); r != nil {
		err := r.(error)
		ExpectString(err.Error(), expectedMsg, t)
		return
	}
	t.Errorf("Expected to panic with `%s`, but did not panic", expectedMsg)
}
