
pub contract TopshotData {
    access(self) var nftMap: {UInt64:NFTData}
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
            TopshotData.nftMap[data.id] = data
            emit NFTDataUpdated(id: data.id, data: data.data)        
        }

        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    pub fun getNFTData(id: UInt64): NFTData? {
        return self.nftMap[id]
    }

    pub init() {
        self.nftMap = {}
        self.account.save<@Admin>(<- create Admin(), to: /storage/TopshotDataAdmin)
    }
}
