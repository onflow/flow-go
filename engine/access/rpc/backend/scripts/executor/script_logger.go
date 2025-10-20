package executor

import (
	"crypto/md5" //nolint:gosec
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"
)

// uniqueScriptLoggingTimeWindow is the duration for checking the uniqueness of scripts sent for execution
const uniqueScriptLoggingTimeWindow = 10 * time.Minute

// ScriptLogger provides deduplicated logging for script execution events.
type ScriptLogger struct {
	log           zerolog.Logger
	loggedScripts *lru.Cache[[md5.Size]byte, time.Time]
}

// NewScriptLogger creates and returns a new [ScriptLogger] instance.
func NewScriptLogger(log zerolog.Logger, loggedScripts *lru.Cache[[md5.Size]byte, time.Time]) *ScriptLogger {
	return &ScriptLogger{
		log:           log,
		loggedScripts: loggedScripts,
	}
}

// LogExecutedScript logs details of a successfully executed script if it has not been
// logged recently.
func (s *ScriptLogger) LogExecutedScript(request *Request, executor string, dur time.Duration) {
	now := time.Now()
	if s.shouldLogScript(now, request.insecureScriptHash) {
		s.log.Debug().
			Str("block_id", request.blockID.String()).
			Str("script_executor_addr", executor).
			Str("script", string(request.script)).
			Dur("execution_dur_ms", dur).
			Msg("Successfully executed script")

		s.loggedScripts.Add(request.insecureScriptHash, now)
	}
}

// LogFailedScript logs details of a failed script execution if it has not been
// logged recently.
func (s *ScriptLogger) LogFailedScript(request *Request, executor string) {
	now := time.Now()
	logEvent := s.log.Debug().
		Str("block_id", request.blockID.String()).
		Str("script_executor_addr", executor)

	if s.shouldLogScript(now, request.insecureScriptHash) {
		logEvent.Str("script", string(request.script))
	}

	logEvent.Msg("failed to execute script")
	s.loggedScripts.Add(request.insecureScriptHash, now)
}

// shouldLogScript determines whether a script should be logged based on its hash
// and the time since it was last logged.
// It returns false if:
//   - The logger’s level is above Debug (i.e., debug logs are disabled), or
//   - The same script hash was logged less than [uniqueScriptLoggingTimeWindow] ago.
func (s *ScriptLogger) shouldLogScript(execTime time.Time, scriptHash [md5.Size]byte) bool {
	if s.log.GetLevel() > zerolog.DebugLevel {
		return false
	}
	timestamp, seen := s.loggedScripts.Get(scriptHash)
	if seen {
		return execTime.Sub(timestamp) >= uniqueScriptLoggingTimeWindow
	}
	return true
}
