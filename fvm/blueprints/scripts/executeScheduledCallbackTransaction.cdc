import "CallbackScheduler"

transaction(callbackID: UInt64) {
    execute {
        CallbackScheduler.executeCallback(ID: callbackID)
    }
}
