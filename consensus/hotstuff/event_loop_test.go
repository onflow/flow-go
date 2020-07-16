package hotstuff

import (
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/module/metrics"
)

func TestReadyDone(t *testing.T) {
	eh := &mocks.EventHandler{}
	eh.On("Start").Return(nil)
	eh.On("TimeoutChannel").Return(time.NewTimer(10 * time.Second).C)
	eh.On("OnLocalTimeout").Return(nil)

	metrics := metrics.NewNoopCollector()

	log := zerolog.New(ioutil.Discard)

	eventLoop, err := NewEventLoop(log, metrics, eh)
	require.NoError(t, err)

	<-eventLoop.Ready()
	time.Sleep(1 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		<-eventLoop.Done()
		wg.Done()
	}()

	// wait until Wait returns
	wg.Wait()
}
