package logging_test

import (
	"bytes"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/logging"
)

// TestComponentLevelWriter_FiltersBelow verifies that events below the configured level
// are silently discarded without writing to the delegate.
func TestComponentLevelWriter_FiltersBelow(t *testing.T) {
	var buf bytes.Buffer
	delegate := logging.NoopLevelWriter(&buf)

	var lvl atomic.Int32
	lvl.Store(int32(zerolog.InfoLevel))

	w := logging.NewComponentLevelWriter(&lvl, delegate)
	n, err := w.WriteLevel(zerolog.DebugLevel, []byte(`{"level":"debug","msg":"hi"}`))
	require.NoError(t, err)
	assert.Equal(t, 0, buf.Len(), "expected nothing written for debug below info")
	assert.Greater(t, n, 0, "expected non-zero n even on discard")
}

// TestComponentLevelWriter_PassesAtOrAbove verifies that events at or above the configured
// level are forwarded to the delegate unchanged.
func TestComponentLevelWriter_PassesAtOrAbove(t *testing.T) {
	var buf bytes.Buffer
	delegate := logging.NoopLevelWriter(&buf)

	var lvl atomic.Int32
	lvl.Store(int32(zerolog.InfoLevel))

	w := logging.NewComponentLevelWriter(&lvl, delegate)
	msg := []byte(`{"level":"warn","msg":"hi"}`)
	_, err := w.WriteLevel(zerolog.WarnLevel, msg)
	require.NoError(t, err)
	assert.Equal(t, msg, buf.Bytes())
}

// TestComponentLevelWriter_AtomicUpdate verifies that updating the atomic level is
// immediately reflected in subsequent writes without re-creating the writer.
func TestComponentLevelWriter_AtomicUpdate(t *testing.T) {
	var buf bytes.Buffer
	delegate := logging.NoopLevelWriter(&buf)

	var lvl atomic.Int32
	lvl.Store(int32(zerolog.InfoLevel))

	w := logging.NewComponentLevelWriter(&lvl, delegate)

	// debug blocked at info
	_, err := w.WriteLevel(zerolog.DebugLevel, []byte(`{"level":"debug"}`))
	require.NoError(t, err)
	assert.Equal(t, 0, buf.Len())

	// lower to debug
	lvl.Store(int32(zerolog.DebugLevel))
	msg := []byte(`{"level":"debug","msg":"now visible"}`)
	_, err = w.WriteLevel(zerolog.DebugLevel, msg)
	require.NoError(t, err)
	assert.Equal(t, msg, buf.Bytes())
}
