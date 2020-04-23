package hotstuff

import (
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestReadyDone(t *testing.T) {
	eh := &mocks.EventHandler{}
	eh.On("Start").Return(nil)
	eh.On("TimeoutChannel").Return(time.NewTimer(10 * time.Second).C)

	metrics := &module.Metrics{}
	log := zerolog.New(ioutil.Discard)

	eventLoop, err := NewEventLoop(log, metrics, eh)
	require.NoError(t, err)

	<-eventLoop.Ready()
	time.Sleep(1 * time.Second)
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		<-eventLoop.Wait()
		wg.Done()
	}()
	<-eventLoop.Done()

	// wait until Wait returns
	wg.Wait()
}
