package backend

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCompare(t *testing.T) {
	m := metrics.NewNoopCollector()
	logger := zerolog.Nop()

	result1 := []byte("result1")
	result2 := []byte("result2")

	error1 := status.Error(codes.InvalidArgument, "error1")
	error2 := status.Error(codes.InvalidArgument, "error2")

	outOfRange := status.Error(codes.OutOfRange, "out of range")

	testcases := []struct {
		name        string
		execResult  *scriptResult
		localResult *scriptResult
		expected    bool
	}{
		{
			name:        "results match",
			execResult:  newScriptResult(result1, 0, nil),
			localResult: newScriptResult(result1, 0, nil),
			expected:    true,
		},
		{
			name:        "results do not match",
			execResult:  newScriptResult(result1, 0, nil),
			localResult: newScriptResult(result2, 0, nil),
			expected:    false,
		},
		{
			name:        "en returns result, local returns error",
			execResult:  newScriptResult(result1, 0, nil),
			localResult: newScriptResult(nil, 0, error1),
			expected:    false,
		},
		{
			name:        "en returns error, local returns result",
			execResult:  newScriptResult(nil, 0, error1),
			localResult: newScriptResult(result1, 0, nil),
			expected:    false,
		},
		{
			// demonstrate this works by passing the same result since the OOR check happens first
			// if the check failed, this should return true
			name:        "local returns out of range",
			execResult:  newScriptResult(result1, 0, nil),
			localResult: newScriptResult(result1, 0, outOfRange),
			expected:    false,
		},
		{
			name:        "both return same error",
			execResult:  newScriptResult(nil, 0, error1),
			localResult: newScriptResult(nil, 0, error1),
			expected:    true,
		},
		{
			name:        "both return different errors",
			execResult:  newScriptResult(nil, 0, error1),
			localResult: newScriptResult(nil, 0, error2),
			expected:    false,
		},
	}

	request := newScriptExecutionRequest(unittest.IdentifierFixture(), 1, []byte("script"), [][]byte{})
	comparer := newScriptResultComparison(logger, m, request)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			actual := comparer.compare(tc.execResult, tc.localResult)
			assert.Equalf(t, tc.expected, actual, "expected %v, got %v", tc.expected, actual)
		})
	}
}

func TestCompareErrors(t *testing.T) {
	testcases := []struct {
		name     string
		execErr  error
		localErr error
		expected bool
	}{
		{
			name:     "both nil",
			execErr:  nil,
			localErr: nil,
			expected: true,
		},
		{
			name:     "same error",
			execErr:  storage.ErrNotFound,
			localErr: storage.ErrNotFound,
			expected: true,
		},
		{
			name:     "same error message",
			execErr:  fmt.Errorf("same error message"),
			localErr: fmt.Errorf("same error message"),
			expected: true,
		},
		{
			name:     "same error code",
			execErr:  status.Error(codes.InvalidArgument, "some message"),
			localErr: status.Error(codes.InvalidArgument, "some message"),
			expected: true,
		},
		{
			name:     "different error code",
			execErr:  status.Error(codes.Canceled, "some message"),
			localErr: status.Error(codes.DeadlineExceeded, "some message"),
			expected: false,
		},
		{
			name:     "same error code, different message",
			execErr:  status.Error(codes.InvalidArgument, "some message"),
			localErr: status.Error(codes.InvalidArgument, "different message"),
			expected: false,
		},
		{
			name:     "same error, different prefix",
			execErr:  status.Errorf(codes.InvalidArgument, "original: %s: some message", executeErrorPrefix),
			localErr: status.Errorf(codes.InvalidArgument, "anything: %s: some message", executeErrorPrefix),
			expected: true,
		},
		{
			name:     "different error, different prefix",
			execErr:  status.Errorf(codes.InvalidArgument, "original: %s: some message", executeErrorPrefix),
			localErr: status.Errorf(codes.InvalidArgument, "anything: %s: another message", executeErrorPrefix),
			expected: false,
		},
		{
			name:     "truncated error, match",
			execErr:  status.Errorf(codes.InvalidArgument, "original: %s: this is the original message", executeErrorPrefix),
			localErr: status.Errorf(codes.InvalidArgument, "anything: %s: this is ... message", executeErrorPrefix),
			expected: true,
		},
		{
			name:     "truncated error, do not match",
			execErr:  status.Errorf(codes.InvalidArgument, "original: %s: this is the original message", executeErrorPrefix),
			localErr: status.Errorf(codes.InvalidArgument, "anything: %s: this is ... a different message", executeErrorPrefix),
			expected: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			actual := compareErrors(tc.execErr, tc.localErr)
			assert.Equalf(t, tc.expected, actual, "expected %v, got %v", tc.expected, actual)
		})
	}
}
