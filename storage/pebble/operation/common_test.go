package operation

import (
	"bytes"
	"testing"
)

func TestGetStartEndKeys(t *testing.T) {
	tests := []struct {
		prefix        []byte
		expectedStart []byte
		expectedEnd   []byte
	}{
		{[]byte("a"), []byte("a"), []byte("b")},
		{[]byte("abc"), []byte("abc"), []byte("abd")},
		{[]byte("prefix"), []byte("prefix"), []byte("prefiy")},
	}

	for _, test := range tests {
		start, end := getStartEndKeys(test.prefix)
		if !bytes.Equal(start, test.expectedStart) {
			t.Errorf("getStartEndKeys(%q) start = %q; want %q", test.prefix, start, test.expectedStart)
		}
		if !bytes.Equal(end, test.expectedEnd) {
			t.Errorf("getStartEndKeys(%q) end = %q; want %q", test.prefix, end, test.expectedEnd)
		}
	}
}
