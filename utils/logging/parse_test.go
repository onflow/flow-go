package logging_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/logging"
)

func TestParseComponentLogLevels_Empty(t *testing.T) {
	result, err := logging.ParseComponentLogLevels("")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseComponentLogLevels_Single(t *testing.T) {
	result, err := logging.ParseComponentLogLevels("hotstuff:debug")
	require.NoError(t, err)
	assert.Equal(t, map[string]zerolog.Level{"hotstuff": zerolog.DebugLevel}, result)
}

func TestParseComponentLogLevels_Multiple(t *testing.T) {
	result, err := logging.ParseComponentLogLevels("hotstuff:debug,network.*:warn")
	require.NoError(t, err)
	assert.Equal(t, map[string]zerolog.Level{
		"hotstuff":  zerolog.DebugLevel,
		"network.*": zerolog.WarnLevel,
	}, result)
}

func TestParseComponentLogLevels_InvalidLevel(t *testing.T) {
	_, err := logging.ParseComponentLogLevels("hotstuff:badlevel")
	require.Error(t, err)
}

func TestParseComponentLogLevels_MissingColon(t *testing.T) {
	_, err := logging.ParseComponentLogLevels("hotstuffdebug")
	require.Error(t, err)
}

func TestParseComponentLogLevels_EmptyComponent(t *testing.T) {
	_, err := logging.ParseComponentLogLevels(":debug")
	require.Error(t, err)
}
