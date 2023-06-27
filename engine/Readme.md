## Block Consumer ([consumer.go](verification%2Fassigner%2Fblockconsumer%2Fconsumer.go))
The `blockconsumer` package efficiently manages the processing of finalized blocks in Verification Node of Flow blockchain.
Specifically, it listens for notifications from the `Follower` engine regarding finalized blocks, and systematically 
queues these blocks for processing. The package employs parallel workers, each an instance of the `Assigner` engine, 
to fetch and process blocks from the queue. The `BlockConsumer` diligently coordinates this process by only assigning 
a new block to a worker once it has completed processing its current block and signaled its availability. 
This ensures that the processing is not only methodical but also resilient to any node crashes. 
In case of a crash, the `BlockConsumer` resumes from where it left off, reassigning blocks from the queue to workers,
thereby guaranteeing no loss of data.

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

