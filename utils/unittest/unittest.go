package unittest

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/topology"
)

type SkipReason int

const (
	TEST_FLAKY               SkipReason = iota + 1 // flaky
	TEST_TODO                                      // not fully implemented or broken and needs to be fixed
	TEST_REQUIRES_GCP_ACCESS                       // requires the environment to be configured with GCP credentials
	TEST_DEPRECATED                                // uses code that has been deprecated / disabled
	TEST_LONG_RUNNING                              // long running
	TEST_RESOURCE_INTENSIVE                        // resource intensive test
)

func (s SkipReason) String() string {
	switch s {
	case TEST_FLAKY:
		return "TEST_FLAKY"
	case TEST_TODO:
		return "TEST_TODO"
	case TEST_REQUIRES_GCP_ACCESS:
		return "TEST_REQUIRES_GCP_ACCESS"
	case TEST_DEPRECATED:
		return "TEST_DEPRECATED"
	case TEST_LONG_RUNNING:
		return "TEST_LONG_RUNNING"
	case TEST_RESOURCE_INTENSIVE:
		return "TEST_RESOURCE_INTENSIVE"
	}
	return "UNKNOWN"
}

func (s SkipReason) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func parseSkipReason(reason string) SkipReason {
	switch reason {
	case "TEST_FLAKY":
		return TEST_FLAKY
	case "TEST_TODO":
		return TEST_TODO
	case "TEST_REQUIRES_GCP_ACCESS":
		return TEST_REQUIRES_GCP_ACCESS
	case "TEST_DEPRECATED":
		return TEST_DEPRECATED
	case "TEST_LONG_RUNNING":
		return TEST_LONG_RUNNING
	case "TEST_RESOURCE_INTENSIVE":
		return TEST_RESOURCE_INTENSIVE
	default:
		return 0
	}
}

func ParseSkipReason(output string) (SkipReason, bool) {
	// match output like:
	// "    test_file.go:123: SKIP [TEST_REASON]: message\n"
	r := regexp.MustCompile(`(?s)^\s+[a-zA-Z0-9_\-]+\.go:[0-9]+: SKIP \[([A-Z_]+)]: .*$`)
	matches := r.FindStringSubmatch(output)

	if len(matches) == 2 {
		skipReason := parseSkipReason(matches[1])
		if skipReason != 0 {
			return skipReason, true
		}
	}

	return 0, false
}

func SkipUnless(t *testing.T, reason SkipReason, message string) {
	t.Helper()
	if os.Getenv(reason.String()) == "" {
		t.Skipf("SKIP [%s]: %s", reason.String(), message)
	}
}

type SkipBenchmarkReason int

const (
	BENCHMARK_EXPERIMENT SkipBenchmarkReason = iota + 1
)

func (s SkipBenchmarkReason) String() string {
	switch s {
	case BENCHMARK_EXPERIMENT:
		return "BENCHMARK_EXPERIMENT"
	}
	return "UNKNOWN"
}

func SkipBenchmarkUnless(b *testing.B, reason SkipBenchmarkReason, message string) {
	b.Helper()
	if os.Getenv(reason.String()) == "" {
		b.Skip(message)
	}
}

// AssertReturnsBefore asserts that the given function returns before the
// duration expires.
func AssertReturnsBefore(t *testing.T, f func(), duration time.Duration, msgAndArgs ...interface{}) bool {
	done := make(chan struct{})

	go func() {
		f()
		close(done)
	}()

	select {
	case <-time.After(duration):
		t.Log("function did not return in time")
		assert.Fail(t, "function did not close in time", msgAndArgs...)
	case <-done:
		return true
	}
	return false
}

// ClosedChannel returns a closed channel.
func ClosedChannel() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// AssertClosesBefore asserts that the given channel closes before the
// duration expires.
func AssertClosesBefore(t assert.TestingT, done <-chan struct{}, duration time.Duration, msgAndArgs ...interface{}) {
	select {
	case <-time.After(duration):
		assert.Fail(t, "channel did not return in time", msgAndArgs...)
	case <-done:
		return
	}
}

func AssertFloatEqual(t *testing.T, expected, actual float64, message string) {
	tolerance := .00001
	if !(math.Abs(expected-actual) < tolerance) {
		assert.Equal(t, expected, actual, message)
	}
}

// AssertNotClosesBefore asserts that the given channel does not close before the duration expires.
func AssertNotClosesBefore(t assert.TestingT, done <-chan struct{}, duration time.Duration, msgAndArgs ...interface{}) {
	select {
	case <-time.After(duration):
		return
	case <-done:
		assert.Fail(t, "channel closed before timeout", msgAndArgs...)
	}
}

// RequireReturnsBefore requires that the given function returns before the
// duration expires.
func RequireReturnsBefore(t testing.TB, f func(), duration time.Duration, message string) {
	done := make(chan struct{})

	go func() {
		f()
		close(done)
	}()

	RequireCloseBefore(t, done, duration, message+": function did not return on time")
}

// RequireComponentsDoneBefore invokes the done method of each of the input components concurrently, and
// fails the test if any components shutdown takes longer than the specified duration.
func RequireComponentsDoneBefore(t testing.TB, duration time.Duration, components ...module.ReadyDoneAware) {
	done := util.AllDone(components...)
	RequireCloseBefore(t, done, duration, "failed to shutdown all components on time")
}

// RequireComponentsReadyBefore invokes the ready method of each of the input components concurrently, and
// fails the test if any components startup takes longer than the specified duration.
func RequireComponentsReadyBefore(t testing.TB, duration time.Duration, components ...module.ReadyDoneAware) {
	ready := util.AllReady(components...)
	RequireCloseBefore(t, ready, duration, "failed to start all components on time")
}

// RequireCloseBefore requires that the given channel returns before the
// duration expires.
func RequireCloseBefore(t testing.TB, c <-chan struct{}, duration time.Duration, message string) {
	select {
	case <-time.After(duration):
		require.Fail(t, "could not close done channel on time: "+message)
	case <-c:
		return
	}
}

// RequireClosed is a test helper function that fails the test if channel `ch` is not closed.
func RequireClosed(t *testing.T, ch <-chan struct{}, message string) {
	select {
	case <-ch:
	default:
		require.Fail(t, "channel is not closed: "+message)
	}
}

// RequireConcurrentCallsReturnBefore is a test helper that runs function `f` count-many times concurrently,
// and requires all invocations to return within duration.
func RequireConcurrentCallsReturnBefore(t *testing.T, f func(), count int, duration time.Duration, message string) {
	wg := &sync.WaitGroup{}
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			f()
			wg.Done()
		}()
	}

	RequireReturnsBefore(t, wg.Wait, duration, message)
}

// RequireNeverReturnBefore is a test helper that tries invoking function `f` and fails the test if either:
// - function `f` is not invoked within 1 second.
// - function `f` returns before specified `duration`.
//
// It also returns a channel that is closed once the function `f` returns and hence its openness can evaluate
// return status of function `f` for intervals longer than duration.
func RequireNeverReturnBefore(t *testing.T, f func(), duration time.Duration, message string) <-chan struct{} {
	ch := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		wg.Done()
		f()
		close(ch)
	}()

	// requires function invoked within next 1 second
	RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not invoke the function: "+message)

	// requires function never returns within duration
	RequireNeverClosedWithin(t, ch, duration, "unexpected return: "+message)

	return ch
}

// RequireNeverClosedWithin is a test helper function that fails the test if channel `ch` is closed before the
// determined duration.
func RequireNeverClosedWithin(t *testing.T, ch <-chan struct{}, duration time.Duration, message string) {
	select {
	case <-time.After(duration):
	case <-ch:
		require.Fail(t, "channel closed before timeout: "+message)
	}
}

// RequireNotClosed is a test helper function that fails the test if channel `ch` is closed.
func RequireNotClosed(t *testing.T, ch <-chan struct{}, message string) {
	select {
	case <-ch:
		require.Fail(t, "channel is closed: "+message)
	default:
	}
}

// AssertErrSubstringMatch asserts that two errors match with substring
// checking on the Error method (`expected` must be a substring of `actual`, to
// account for the actual error being wrapped). Fails the test if either error
// is nil.
//
// NOTE: This should only be used in cases where `errors.Is` cannot be, like
// when errors are transmitted over the network without type information.
func AssertErrSubstringMatch(t testing.TB, expected, actual error) {
	require.NotNil(t, expected)
	require.NotNil(t, actual)
	assert.True(
		t,
		strings.Contains(actual.Error(), expected.Error()) || strings.Contains(expected.Error(), actual.Error()),
		"expected error: '%s', got: '%s'", expected.Error(), actual.Error(),
	)
}

func TempDir(t testing.TB) string {
	dir, err := os.MkdirTemp("", "flow-testing-temp-")
	require.NoError(t, err)
	return dir
}

func RunWithTempDir(t testing.TB, f func(string)) {
	dbDir := TempDir(t)
	defer func() {
		require.NoError(t, os.RemoveAll(dbDir))
	}()
	f(dbDir)
}

func badgerDB(t testing.TB, dir string, create func(badger.Options) (*badger.DB, error)) *badger.DB {
	opts := badger.
		DefaultOptions(dir).
		WithKeepL0InMemory(true).
		WithLogger(nil)
	db, err := create(opts)
	require.NoError(t, err)
	return db
}

func BadgerDB(t testing.TB, dir string) *badger.DB {
	return badgerDB(t, dir, badger.Open)
}

func TypedBadgerDB(t testing.TB, dir string, create func(badger.Options) (*badger.DB, error)) *badger.DB {
	return badgerDB(t, dir, create)
}

func RunWithBadgerDB(t testing.TB, f func(*badger.DB)) {
	RunWithTempDir(t, func(dir string) {
		db := BadgerDB(t, dir)
		defer func() {
			assert.NoError(t, db.Close())
		}()
		f(db)
	})
}

// RunWithTypedBadgerDB creates a Badger DB that is passed to f and closed
// after f returns. The extra create parameter allows passing in a database
// constructor function which instantiates a database with a particular type
// marker, for testing storage modules which require a backed with a particular
// type.
func RunWithTypedBadgerDB(t testing.TB, create func(badger.Options) (*badger.DB, error), f func(*badger.DB)) {
	RunWithTempDir(t, func(dir string) {
		db := badgerDB(t, dir, create)
		defer func() {
			assert.NoError(t, db.Close())
		}()
		f(db)
	})
}

func TempBadgerDB(t testing.TB) (*badger.DB, string) {
	dir := TempDir(t)
	db := BadgerDB(t, dir)
	return db, dir
}

func TempPebblePath(t *testing.T) string {
	return path.Join(TempDir(t), "pebble"+strconv.Itoa(rand.Int())+".db")
}

func TempPebbleDBWithOpts(t testing.TB, opts *pebble.Options) (*pebble.DB, string) {
	// create random path string for parallelization
	dbpath := path.Join(TempDir(t), "pebble"+strconv.Itoa(rand.Int())+".db")
	db, err := pebble.Open(dbpath, opts)
	require.NoError(t, err)
	return db, dbpath
}

func Concurrently(n int, f func(int)) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			f(i)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

// AssertEqualBlocksLenAndOrder asserts that both a segment of blocks have the same len and blocks are in the same order
func AssertEqualBlocksLenAndOrder(t *testing.T, expectedBlocks, actualSegmentBlocks []*flow.Block) {
	assert.Equal(t, flow.GetIDs(expectedBlocks), flow.GetIDs(actualSegmentBlocks))
}

// NetworkCodec returns cbor codec.
func NetworkCodec() network.Codec {
	return cborcodec.NewCodec()
}

// NetworkTopology returns the default topology for testing purposes.
func NetworkTopology() network.Topology {
	return topology.NewFullyConnectedTopology()
}

// CrashTest safely tests functions that crash (as the expected behavior) by checking that running the function creates an error and
// an expected error message.
func CrashTest(t *testing.T, scenario func(*testing.T), expectedErrorMsg string) {
	CrashTestWithExpectedStatus(t, scenario, expectedErrorMsg, 1)
}

// CrashTestWithExpectedStatus checks for the test crashing with a specific exit code.
func CrashTestWithExpectedStatus(
	t *testing.T,
	scenario func(*testing.T),
	expectedErrorMsg string,
	expectedStatus ...int,
) {
	require.NotNil(t, scenario)
	require.NotEmpty(t, expectedStatus)

	if os.Getenv("CRASH_TEST") == "1" {
		scenario(t)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(), "CRASH_TEST=1")

	outBytes, err := cmd.Output()
	// expect error from run
	require.Error(t, err)

	// expect specific status codes
	require.Contains(t, expectedStatus, cmd.ProcessState.ExitCode())

	// expect logger.Fatal() message to be pushed to stdout
	outStr := string(outBytes)
	require.Contains(t, outStr, expectedErrorMsg)
}

// GenerateRandomStringWithLen returns a string of random alpha characters of the provided length
func GenerateRandomStringWithLen(commentLen uint) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := make([]byte, commentLen)
	for i := range bytes {
		bytes[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(bytes)
}

// PeerIdFixture creates a random and unique peer ID (libp2p node ID).
func PeerIdFixture(tb testing.TB) peer.ID {
	peerID, err := peerIDFixture()
	require.NoError(tb, err)
	return peerID
}

func peerIDFixture() (peer.ID, error) {
	key, err := generateNetworkingKey(IdentifierFixture())
	if err != nil {
		return "", err
	}
	pubKey, err := keyutils.LibP2PPublicKeyFromFlow(key.PublicKey())
	if err != nil {
		return "", err
	}

	peerID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return "", err
	}

	return peerID, nil
}

// generateNetworkingKey generates a Flow ECDSA key using the given seed
func generateNetworkingKey(s flow.Identifier) (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	copy(seed, s[:])
	return crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
}

// PeerIdFixtures creates random and unique peer IDs (libp2p node IDs).
func PeerIdFixtures(t *testing.T, n int) []peer.ID {
	peerIDs := make([]peer.ID, n)
	for i := 0; i < n; i++ {
		peerIDs[i] = PeerIdFixture(t)
	}
	return peerIDs
}
