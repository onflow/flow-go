# Notifier

The Notifier implements the following state machine
![Notifier State Machine](/docs/NotifierStateMachine.png)

The intended usage pattern is:
* there are goroutines, aka `Producer`s, that append work to a queue `pendingWorkQueue`
* there is a number of goroutines, aka `Consumer`s, that pull work from the `pendingWorkQueue`
   * they consume work until they have drained the `pendingWorkQueue`
   * when they find that the `pendingWorkQueue` contains no more work, they go back to 
     the notifier and await notification 

![Notifier Usage Pattern](/docs/NotifierUsagePattern.png)

Note that the consumer / producer interact in a _different_ order with the `pendingWorkQueue` vs the `notifier`:
* the producer first drops its work into the queue and subsequently sends the notification 
* the consumer first processes elements from the queue and subsequently checks for a notification 
Thereby, it is guaranteed that at least one consumer routine will be notified when work is added

