access(all) contract ComputationHeavy {
    access(all) fun ComputationHeavy(_ n: Int) {
    	var s: Int256 = 1024102410241024
        var i = 0
        var a = Int256(7)
        var b = Int256(5)
        var c = Int256(2)
        while i < n {
            s = s * a
            s = s / b
            s = s / c
            i = i + 1
        }
        log(i)
    }
}
