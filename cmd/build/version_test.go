package build

import "testing"

func TestMakeSemverV2Compliant(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"No hyphen", "v0.29", "0.29.0"},
		{"With hyphen", "v0.29.11-an-error-handling", "0.29.11-an-error-handling"},
		{"With hyphen no patch", "v0.29-an-error-handling", "0.29.0-an-error-handling"},
		{"All digits", "v0.29.1", "0.29.1"},
		{undefined, undefined, undefined},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := makeSemverCompliant(tc.input)
			if output != tc.expected {
				t.Errorf("Got %s; expected %s", output, tc.expected)
			}
		})
	}
}
