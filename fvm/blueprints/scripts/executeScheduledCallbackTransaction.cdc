import "UnsafeCallbackScheduler"

transaction(callbackID: UInt64) {
    execute {
        UnsafeCallbackScheduler.executeCallback(ID: callbackID)
    }
}
