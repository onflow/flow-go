access(all) contract EventHeavy {
    access(all) event LargeEvent(value: Int256, str: String, list: [UInt256], dic: {String: String})

    access(all) fun EventHeavy(_ n: Int) {
        var s: Int256 = 1024102410241024
        var i = 0

        while i < n {
            emit LargeEvent(value: s, str: s.toString(), list:[], dic:{s.toString():s.toString()})
            i = i + 1
        }
        log(i)
    }
}
