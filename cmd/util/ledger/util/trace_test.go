package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrace(t *testing.T) {
	t.Parallel()

	trace := NewTrace("foo")
	assert.Equal(t, "foo", trace.String())

	traceBar := trace.Append(".bar")
	assert.Equal(t, "foo.bar", traceBar.String())

	traceBarBaz := traceBar.Append(".baz")
	assert.Equal(t, "foo.bar.baz", traceBarBaz.String())

	traceQux := trace.Append(".qux")
	assert.Equal(t, "foo.qux", traceQux.String())

	traceQuux := traceQux.Append(".quux")
	assert.Equal(t, "foo.qux.quux", traceQuux.String())
}
