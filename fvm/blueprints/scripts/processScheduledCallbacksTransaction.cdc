import "CallbackScheduler"

transaction(maxEffortLeft: UInt64) {
    execute {
        CallbackScheduler.process(maxEffortLeft: effortLimit)
    }
}
