pub contract SomePlaceCounter {
    pub var nextSetID: UInt64
    
    access(account) fun incrementSetCounter(): UInt64 {
        self.nextSetID = self.nextSetID + 1
        return self.nextSetID
    }

    init() {
        self.nextSetID = 1
    }
}