package executor

import (
	"crypto/md5"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

type LoggedScriptCache struct {
	log           zerolog.Logger
	loggedScripts *lru.Cache[[md5.Size]byte, time.Time]
}

func NewScriptCache(log zerolog.Logger, loggedScripts *lru.Cache[[md5.Size]byte, time.Time]) *LoggedScriptCache {
	return &LoggedScriptCache{
		log:           log,
		loggedScripts: loggedScripts,
	}
}

func (s *LoggedScriptCache) LogExecutedScript(
	blockID flow.Identifier,
	scriptHash [md5.Size]byte,
	executionTime time.Time,
	address string,
	script []byte,
	dur time.Duration,
) {
	if s.shouldLogScript(executionTime, scriptHash) {
		s.log.Debug().
			Str("block_id", blockID.String()).
			Str("script_executor_addr", address).
			Str("script", string(script)).
			Dur("execution_dur_ms", dur).
			Msg("Successfully executed script")

		s.loggedScripts.Add(scriptHash, executionTime)
	}
}

func (s *LoggedScriptCache) LogFailedScript(
	blockID flow.Identifier,
	scriptHash [md5.Size]byte,
	executionTime time.Time,
	address string,
	script []byte,
) {
	if s.shouldLogScript(executionTime, scriptHash) {
		s.log.Debug().
			Str("block_id", blockID.String()).
			Str("script_executor_addr", address).
			Str("script", string(script)).
			Msg("Successfully executed script")

		s.loggedScripts.Add(scriptHash, executionTime)
	}
}

func (s *LoggedScriptCache) shouldLogScript(execTime time.Time, scriptHash [md5.Size]byte) bool {
	if s.log.GetLevel() > zerolog.DebugLevel {
		return false
	}
	timestamp, seen := s.loggedScripts.Get(scriptHash)
	if seen {
		return execTime.Sub(timestamp) >= uniqueScriptLoggingTimeWindow
	}
	return true
}
