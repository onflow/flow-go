package backend

import (
	"bytes"
	"crypto/md5" //nolint:gosec
	"encoding/base64"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/module"
)

const (
	executeErrorPrefix = "failed to execute script at block"
	logDiffAsError     = false
)

type scriptResult struct {
	result   []byte
	duration time.Duration
	err      error
}

func newScriptResult(result []byte, duration time.Duration, err error) *scriptResult {
	return &scriptResult{
		result:   result,
		duration: duration,
		err:      err,
	}
}

type scriptResultComparison struct {
	log             zerolog.Logger
	metrics         module.BackendScriptsMetrics
	request         *scriptExecutionRequest
	shouldLogScript func(time.Time, [md5.Size]byte) bool
}

func newScriptResultComparison(
	log zerolog.Logger,
	metrics module.BackendScriptsMetrics,
	shouldLogScript func(time.Time, [md5.Size]byte) bool,
	request *scriptExecutionRequest,
) *scriptResultComparison {
	return &scriptResultComparison{
		log:             log,
		metrics:         metrics,
		request:         request,
		shouldLogScript: shouldLogScript,
	}
}

func (c *scriptResultComparison) compare(execResult, localResult *scriptResult) bool {
	// record errors caused by missing local data
	if isOutOfRangeError(localResult.err) {
		c.metrics.ScriptExecutionNotIndexed()
		c.logComparison(execResult, localResult,
			"script execution results do not match EN because data is not indexed yet", false)
		return false
	}

	// check errors first
	if execResult.err != nil {
		if compareErrors(execResult.err, localResult.err) {
			c.metrics.ScriptExecutionErrorMatch()
			return true
		}

		c.metrics.ScriptExecutionErrorMismatch()
		c.logComparison(execResult, localResult,
			"cadence errors from local execution do not match EN", logDiffAsError)
		return false
	}

	if bytes.Equal(execResult.result, localResult.result) {
		c.metrics.ScriptExecutionResultMatch()
		return true
	}

	c.metrics.ScriptExecutionResultMismatch()
	c.logComparison(execResult, localResult,
		"script execution results from local execution do not match EN", logDiffAsError)
	return false
}

// logScriptExecutionComparison logs the script execution comparison between local execution and execution node
func (c *scriptResultComparison) logComparison(execResult, localResult *scriptResult, msg string, useError bool) {
	args := make([]string, len(c.request.arguments))
	for i, arg := range c.request.arguments {
		args[i] = string(arg)
	}

	lgCtx := c.log.With().
		Hex("block_id", c.request.blockID[:]).
		Hex("script_hash", c.request.insecureScriptHash[:]).
		Strs("args", args)

	if c.shouldLogScript(time.Now(), c.request.insecureScriptHash) {
		lgCtx = lgCtx.Str("script", string(c.request.script))
	}

	if execResult.err != nil {
		lgCtx = lgCtx.AnErr("execution_node_error", execResult.err)
	} else {
		lgCtx = lgCtx.Str("execution_node_result", base64.StdEncoding.EncodeToString(execResult.result))
	}
	lgCtx = lgCtx.Dur("execution_node_duration_ms", execResult.duration)

	if localResult.err != nil {
		lgCtx = lgCtx.AnErr("local_error", localResult.err)
	} else {
		lgCtx = lgCtx.Str("local_result", base64.StdEncoding.EncodeToString(localResult.result))
	}
	lgCtx = lgCtx.Dur("local_duration_ms", localResult.duration)

	lg := lgCtx.Logger()
	if useError {
		lg.Error().Msg(msg)
	} else {
		lg.Debug().Msg(msg)
	}
}

func isOutOfRangeError(err error) bool {
	return status.Code(err) == codes.OutOfRange
}

func compareErrors(execErr, localErr error) bool {
	if execErr == localErr {
		return true
	}

	// if the status code is different, then they definitely don't match
	if status.Code(execErr) != status.Code(localErr) {
		return false
	}

	// absolute error strings generally won't match since the code paths are slightly different
	// check if the original error is the same by removing unneeded error wrapping.
	return containsError(execErr, localErr)
}

func containsError(execErr, localErr error) bool {
	// both script execution implementations use the same engine, which adds
	// "failed to execute script at block" to the message before returning. Any characters
	// before this can be ignored. The string that comes after is the original error and
	// should match.
	execErrStr := trimErrorPrefix(execErr)
	localErrStr := trimErrorPrefix(localErr)

	if execErrStr == localErrStr {
		return true
	}

	// by default ENs are configured with longer script error size limits, which means that the AN's
	// error may be truncated. check if the non-truncated parts match.
	subParts := strings.Split(localErrStr, " ... ")

	return len(subParts) == 2 &&
		strings.HasPrefix(execErrStr, subParts[0]) &&
		strings.HasSuffix(execErrStr, subParts[1])
}

func trimErrorPrefix(err error) string {
	if err == nil {
		return ""
	}

	parts := strings.Split(err.Error(), executeErrorPrefix)
	if len(parts) != 2 {
		return err.Error()
	}

	return parts[1]
}
