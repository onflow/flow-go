package meter

type Meter interface {
	// merge child funcionality
	NewChild() Meter
	MergeMeter(child Meter) error

	// computation metering
	MeterComputation(kind uint, intensity uint) error
	TotalComputationUsed() uint
	TotalComputationLimit() uint

	// memory metering
	MeterMemory(kind uint, intensity uint) error
	TotalMemoryUsed() uint
	TotalMemoryLimit() uint

	// TODO move storage metering to here
	// MeterStorageRead(byteSize uint) error
	// MeterStorageWrite(byteSize uint) error
	// TotalBytesReadFromStorage() int
	// TotalBytesWroteToStorage() int
	// TotalBytesOfStorageInteractions() int
}
