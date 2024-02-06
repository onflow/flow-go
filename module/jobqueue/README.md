## JobQueue Design Goal

The jobqueue package implemented a reusable job queue system for async message processing.

The most common use case is to work on each finalized block async.

For instance, verification nodes must verify each finalized block. This needs to happen async, otherwise a verification node might get overwhelmed during periods when a large amount of blocks are finalized quickly (e.g. when a node comes back online and is catching up from other peers).

So the goal for the jobqueue system are:
1. guarantee each job (i.e. finalized block) will be processed eventually
2. in the event of a crash failure, the jobqueue state is persisted and workers can be rescheduled so that no job is skipped.
3. allow concurrent processing of multiple jobs
4. the number of concurrent workers is configurable so that the node won't get overwhelmed when too many jobs are created (i.e. too many blocks are finalized in a short period of time)

## JobQueue components
To achieve the above goal, the jobqueue system contains the following components/interfaces:
1. A `Jobs` module to find jobs by job index
2. A `storage.ConsumerProgress` to store job processing progress
3. A `Worker` module to process jobs and report job completion.
4. A `Consumer` that orchestrates the job processing by finding new jobs, creating workers for each job using the above modules, and managing job processing status internally.

### Using module.Jobs to find jobs
There is no JobProducer in jobqueue design. Job queue assumes each job can be indexed by a uint64 value, just like each finalized block (or sealed block) can be indexed by block height.

Let's just call this uint64 value "Job Index" or index.

So if we iterate through each index from low to high, and find each job by index, then we are able to iterate through each job.

Therefore modules.Job interface abstracts it into a method: `AtIndex`.

`AtIndex` method returns the job at any given index.

Job consumer relies on the modules.Jobs to find jobs. However, modules.Jobs doesn't provide a way to notify as soon as a new job is available. So it's consumer's job to keep track of the values returned by module.Jobs's `Head` method and find jobs that are new.

### Using Check method to notify job consumer for checking new jobs
Job consumer provides the `Check` method for users to notify new jobs available.

Once called, job consumer will iterate through each height with the `AtIndex` method. It stops when one of the following condition is true:
1. no job was found at a index
2. no more workers to work on them, which is limited by the config item `maxProcessing`

`Check` method is concurrent safe, meaning even if job consumer is notified concurrently about new jobs available, job consumer will check at most once to find new jobs.

Whenever a worker finishes a job, job consumer will also call `Check` internally.

### Storing job consuming progress in storage.ConsumerProgress
Job consumer stores the last processed job index in `storage.ConsumerProgress`, so that on startup, the job consumer can read the last processed job index from storage and compare with the last available job index from `module.Jobs`'s `Head` method to resume job processing.

This ensures each job will be processed at least once. Note: given the at least once execution, the `Worker` should gracefully handle duplicate runs of the same job.

### Using Workers to work on each job

When Job consumer finds a new job, it uses an implementation of the `Worker` interface to process each job. The `Worker`s `Run` method accepts a `module.Job` interface. So it's the user's responsibility to handle the conversion between `module.Job` and the underlying data type.

In the scenario of processing finalized blocks, implementing symmetric functions like BlockToJob and JobToBlock are recommended for this conversion.

In order to report job completion, the worker needs to call job consumer's `NotifyJobIsDone` method.

### Error handling
Job queue doesn't allow job to fail, because job queue has to guarantee any job below the last processed job index has been finished successfully. Leaving a gap is not accpeted.

Therefore, if a worker fails to process a job, it should retry by itself, or just crash.

Note, Worker should not log the error and report the job is completed, because that would change the last processed job index, and will not have the chance to retry that job.


## Pipeline Pattern
Multiple jobqueues can be combined to form a pipeline. This is useful in the scenario that the first job queue will process each finalized block and create jobs to process data depending on the block, and having the second job queue to process each job created by the worker of the first job queue.

For instance, verification node uses 2-jobqueue pipeline to find chunks from each block and create jobs if the block has chunks that it needs to verify, and the second job queue will allow verification node to verify each chunk with a max number of workers.

## Considerations

### Push vs Pull
The jobqueue architecture is optimized for "pull" style processes, where the job producer simply notify the job consumer about new jobs without creating any job, and job consumer pulls jobs from a source when workers are available. All current implementations are using this pull style since it lends well to asynchronously processing jobs based on block heights.

Some use cases might require "push" style jobs where there is a job producer that create new jobs, and a consumer that processes work from the producer. This is possible with the jobqueue, but requires the producer persist the jobs to a database, then implement the `Head` and `AtIndex` methods that allow accessing jobs by sequential `uint64` indexes.

### TODOs
1. Jobs at different index are processed in parallel, it's possible that there is a job takes a long time to work on, and causing too many completed jobs cached in memory before being used to update the the last processed job index.
  `maxSearchAhead` will allow the job consumer to stop consume more blocks if too many jobs are completed, but the job at index lastProcesssed + 1 has not been unprocessed yet.
  The difference between `maxSearchAhead` and `maxProcessing` is that: `maxProcessing` allows at most `maxProcessing` number of works to process jobs. However, even if there is worker available, it might not be assigned to a job, because the job at index lastProcesssed +1 has not been done, it won't work on an job with index higher than `lastProcesssed + maxSearchAhead`.
2. accept callback to get notified when the consecutive job index is finished.
3. implement ReadyDoneAware interface

