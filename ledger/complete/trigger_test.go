package complete

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/common"
	mocks "github.com/onflow/flow-go/ledger/complete/mock"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func TestHasEnoughSegmentsToTriggerCheckpoint(t *testing.T) {
	c := &Trigger{checkpointedSegumentNum: atomic.NewInt32(0), checkpointDistance: 3}
	require.False(t, c.hasEnoughSegmentsToTriggerCheckpoint(1))
	require.False(t, c.hasEnoughSegmentsToTriggerCheckpoint(2))
	require.True(t, c.hasEnoughSegmentsToTriggerCheckpoint(3))
	require.True(t, c.hasEnoughSegmentsToTriggerCheckpoint(4))
}

func createNTries(n int) []*trie.MTrie {
	emptyTrie := trie.NewEmptyMTrie()
	tries := make([]*trie.MTrie, 0, n)
	for i := 0; i < n; i++ {
		path := testutils.PathByUint16LeftPadded(uint16(i))
		t, _, _ := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*testutils.LightPayload(1, 2)}, true)
		tries = append(tries, t)
	}
	return tries
}

// should trigger checkpoint if checkpointDistance number of segment num is created,
// should not trigger checkpoint if not enough checkpointDistance number of segment num is created
func TestTriggerNotifyTrieUpdateWrittenToWAL(t *testing.T) {
	// create trigger with configs
	activeSegmentNum := 10
	checkpointedSegumentNum := 9
	checkpointDistance := 2

	runner := &mocks.CheckpointRunner{}
	runner.On("RunCheckpoint", mock.Anything, mock.Anything, mock.Anything).Return(nil).Twice()
	trigger := NewTrigger(zerolog.Logger{}, activeSegmentNum, checkpointedSegumentNum, uint(checkpointDistance), runner, nil, atomic.NewBool(false))

	// prepare for tries
	tries := createNTries(10)
	queue := common.NewTrieQueue(10)
	queue.Push(tries[0])
	ctx := context.Background()

	var segmentNum int
	var err error
	// trie update written to the same segment as the active segment num will not trigger checkpoint
	segmentNum = 10
	err = trigger.NotifyTrieUpdateWrittenToWAL(ctx, segmentNum, queue)
	require.NoError(t, err)

	// trie update is writting to a different segment file, it means we have finish writting to segment 10,
	// but it still won't trigger checkpoint, because the checkpointDistance is 2 and last checkpointed num is 9,
	// it won't trigger until we finish writing to segment 11
	segmentNum = 11
	err = trigger.NotifyTrieUpdateWrittenToWAL(ctx, segmentNum, queue)
	require.NoError(t, err)

	// another trie update to the same file still won't trigger checkpoint
	segmentNum = 11
	err = trigger.NotifyTrieUpdateWrittenToWAL(ctx, segmentNum, queue)
	require.NoError(t, err)

	time.Sleep(1 * time.Millisecond)
	// up until now, RunCheckpoint should never triggered
	runner.AssertNotCalled(t, "RunCheckpoint")

	// trie update is writting to a different segment file, it means we have finish writting to previous segment 11,
	// since segment 11 reaches the checkpointDistance (2) from last checkpointed segment num 9, so it should trigger
	// checkpoint
	segmentNum = 12
	err = trigger.NotifyTrieUpdateWrittenToWAL(ctx, segmentNum, queue)
	require.NoError(t, err)

	time.Sleep(1 * time.Millisecond) // checkpoint runs on a separate goroutine, we have to wait
	runner.AssertCalled(t, "RunCheckpoint", ctx, queue.Tries(), 11)

	// now checkpointed segment num should be updated to 11, it should not trigger until finish
	// writing to 13
	segmentNum = 13
	err = trigger.NotifyTrieUpdateWrittenToWAL(ctx, segmentNum, queue)
	require.NoError(t, err)

	time.Sleep(1 * time.Millisecond)
	// up until now, RunCheckpoint should never triggered
	runner.AssertNotCalled(t, "RunCheckpoint")

	segmentNum = 14
	err = trigger.NotifyTrieUpdateWrittenToWAL(ctx, segmentNum, queue)
	require.NoError(t, err)
	time.Sleep(1 * time.Millisecond)
	runner.AssertCalled(t, "RunCheckpoint", ctx, queue.Tries(), 13)

	// in total RunCheckpoint should only called once
	runner.AssertExpectations(t)
}

func TestTriggerOnlyOneCheckpointerIsRunning(t *testing.T) {
	// create trigger with configs
	activeSegmentNum := 10
	checkpointedSegumentNum := 9
	checkpointDistance := 2

	runner := &mocks.CheckpointRunner{}
	runner.On("RunCheckpoint", mock.Anything, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, checkpointTries []*trie.MTrie, checkpointNum int) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		}).Twice()
	trigger := NewTrigger(zerolog.Logger{}, activeSegmentNum, checkpointedSegumentNum, uint(checkpointDistance), runner, nil, atomic.NewBool(false))

	// prepare for tries
	tries := createNTries(10)
	queue := common.NewTrieQueue(10)
	queue.Push(tries[0])
	ctx := context.Background()

	// each time should trigger a checkpoint check, since distance is 1, checkpoint should be triggered
	// all the time.
	err := trigger.NotifyTrieUpdateWrittenToWAL(ctx, activeSegmentNum+1, queue)
	require.NoError(t, err)

	// ensure checkpointing has started
	time.Sleep(1 * time.Millisecond)

	for i := 12; i < 32; i++ {
		err := trigger.NotifyTrieUpdateWrittenToWAL(ctx, i, queue)
		require.NoError(t, err)
		// since the checkpointing will last for 10 Millisecond, it should only trigger once.
	}

	time.Sleep(1 * time.Millisecond)
	runner.AssertCalled(t, "RunCheckpoint", ctx, queue.Tries(), 11) // 9 + 2

	// wait long enough that checkpoint has finished and trigger
	time.Sleep(30 * time.Millisecond)
	err = trigger.NotifyTrieUpdateWrittenToWAL(ctx, 32, queue)
	require.NoError(t, err)

	// ensure checkpointing has started
	time.Sleep(1 * time.Millisecond)
	runner.AssertCalled(t, "RunCheckpoint", ctx, queue.Tries(), 31)
	runner.AssertExpectations(t) // check in total RunCheckpoint was only called twice
}
