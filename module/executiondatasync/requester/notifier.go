package requester

import (
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
)

type notificationSub struct {
	consumer BlockExecutionDataConsumer
}

type notification struct {
	blockHeight   uint64
	executionData *execution_data.BlockExecutionData
}

type notifier struct {
	notificationsIn  chan<- interface{}
	notificationsOut <-chan interface{}

	subs   chan *notificationSub
	unsubs chan *notificationSub

	subscriptions map[*notificationSub]struct{}

	component.Component
}

func newNotifier() *notifier {
	notificationsIn, notificationsOut := util.UnboundedChannel()
	n := &notifier{
		notificationsIn:  notificationsIn,
		notificationsOut: notificationsOut,
		subs:             make(chan *notificationSub),
		unsubs:           make(chan *notificationSub),
		subscriptions:    make(map[*notificationSub]struct{}),
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(n.loop).
		Build()
	n.Component = cm

	return n
}

func (n *notifier) notify(blockHeight uint64, executionData *execution_data.BlockExecutionData) {
	n.notificationsIn <- &notification{blockHeight, executionData}
}

func (n *notifier) subscribe(sub *notificationSub) func() {
	n.subs <- sub
	return func() {
		n.unsubs <- sub
	}
}

func (n *notifier) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case sub := <-n.subs:
			n.subscriptions[sub] = struct{}{}
		case sub := <-n.unsubs:
			delete(n.subscriptions, sub)
		case notif := <-n.notificationsOut:
			notification := notif.(*notification)
			for sub := range n.subscriptions {
				sub.consumer(notification.blockHeight, notification.executionData)
			}
		}
	}
}
