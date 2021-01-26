package module

type JobID string

type JobConsumer interface {
	Start() error

	Stop()

	FinishJob(JobID)

	Check()
}
