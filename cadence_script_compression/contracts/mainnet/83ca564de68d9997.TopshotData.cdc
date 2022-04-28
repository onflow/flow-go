pub contract TopshotData {
    access(self) var shardedNFTMap: {UInt64: {UInt64: NFTData}}
    pub let numBuckets: UInt64

    pub event NFTDataUpdated(id: UInt64, data: {String: String})

    pub struct NFTData{
        pub let data: {String:String}
        pub let id: UInt64
        init(id: UInt64, data: {String:String}) {
            self.id = id
            self.data = data
        }
    }

    pub resource Admin {

        pub fun upsertNFTData(data: NFTData) {
           let bucket = data.id % TopshotData.numBuckets
		   let nftMap = TopshotData.shardedNFTMap[bucket]!
           nftMap[data.id] = data
           emit NFTDataUpdated(id: data.id, data: data.data)        
        }
		
        pub fun createNewAdmin(): @Admin {
           return <-create Admin()
        }
		
		pub init() {}
    }

    pub fun getNFTData(id: UInt64): NFTData? {
        let bucket = id % TopshotData.numBuckets
        return self.shardedNFTMap[bucket]![id]
    }

    pub init() {
        self.numBuckets = UInt64(100)
        self.shardedNFTMap = {}
        var i: UInt64 = 0
        while i < self.numBuckets {
            self.shardedNFTMap[i] = {}
            i = i + UInt64(1)
        }
        self.account.save<@Admin>(<- create Admin(), to: /storage/TopshotDataAdminV2)
    }
}
