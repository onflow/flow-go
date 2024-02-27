package backend

import (
	"bytes"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/module"
)

const (
	executeErrorPrefix = "failed to execute script at block"
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
	log     zerolog.Logger
	metrics module.BackendScriptsMetrics
	request *scriptExecutionRequest
}

func newScriptResultComparison(
	log zerolog.Logger,
	metrics module.BackendScriptsMetrics,
	request *scriptExecutionRequest,
) *scriptResultComparison {
	return &scriptResultComparison{
		log:     log,
		metrics: metrics,
		request: request,
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
			c.logDurationDifference(execResult, localResult)
			return true
		}

		c.metrics.ScriptExecutionErrorMismatch()
		c.logComparison(execResult, localResult,
			"cadence errors from local execution do not match EN", true)
		return false
	}

	if bytes.Equal(execResult.result, localResult.result) {
		c.metrics.ScriptExecutionResultMatch()
		c.logDurationDifference(execResult, localResult)
		return true
	}

	c.metrics.ScriptExecutionResultMismatch()
	c.logComparison(execResult, localResult,
		"script execution results from local execution do not match EN", true)
	return false
}

// logDurationDifference logs the script execution details for local execution and execution node if the difference is
// more than 2x
func (c *scriptResultComparison) logDurationDifference(execResult *scriptResult, localResult *scriptResult) {

	if execResult.duration.Milliseconds() == 0 || localResult.duration.Milliseconds() == 0 {
		return
	}

	speedup := float64(localResult.duration.Milliseconds()) / float64(execResult.duration.Milliseconds())
	// if the script execution on execution node was more than 2x faster
	if speedup > 2.0 {
		c.logComparison(execResult, localResult,
			"access node script execution was slower", true)
	} else {
		// if the script execution on the access node was more than 2x faster
		if speedup < 0.5 {
			c.logComparison(execResult, localResult,
				"access node script execution was faster", true)
		}
	}
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
		Str("script", string(c.request.script)).
		Strs("args", args)

	if execResult.err != nil {
		lgCtx = lgCtx.AnErr("execution_node_error", execResult.err)
	} else {
		lgCtx = lgCtx.Hex("execution_node_result", execResult.result)
	}
	lgCtx = lgCtx.Dur("execution_node_duration_ms", execResult.duration)

	if localResult.err != nil {
		lgCtx = lgCtx.AnErr("local_error", localResult.err)
	} else {
		lgCtx = lgCtx.Hex("local_result", localResult.result)
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
