pub contract VnMissCandidate {
    access(self) let listCandidate: { UInt64: Candidate }
    access(self) let top40: { UInt64: Bool }

    pub let AdminStoragePath: StoragePath
    pub let MaxCandidate: Int

    pub event NewCandidate(id: UInt64, name: String, fundAddress: Address)
    pub event CandidateUpdate(id: UInt64, name: String, fundAddress: Address)

    pub struct Candidate {
        pub let id: UInt64
        pub let name: String
        pub let code: String
        pub let description: String
        pub let fundAdress: Address
        pub let properties: {String: String}

        init(
            id: UInt64,
            name: String,
            code: String,
            description: String,
            fundAddress: Address,
            properties: {String: String}
        ) {
            self.id = id
            self.name = name
            self.code = code
            self.description = description
            self.fundAdress = fundAddress
            self.properties = properties
        }

        pub fun buildName(level: String, id: UInt64): String {
            return  self.name.concat(" ")
                        .concat(self.code)
                        .concat(" - ")
                        .concat(level)
                        .concat("#")
                        .concat(id.toString())
        }

        pub fun inTop40(): Bool {
            return VnMissCandidate.top40[self.id] ?? false
        }
    }

    pub resource Admin {
        pub fun createCandidate(
            id: UInt64,
            name: String,
            code: String,
            description: String,
            fundAddress: Address,
            properties: {String: String}
        ) {
            pre {
                VnMissCandidate.listCandidate.length < VnMissCandidate.MaxCandidate: "Exceed maximum"
            }
            
            VnMissCandidate.listCandidate[id] = Candidate(id: id, name: name, code: code, description: description, fundAddress: fundAddress, properties: properties)
            emit NewCandidate(id: id, name: name, fundAddress: fundAddress)
        }

        pub fun markTop40(ids: [UInt64], isTop40: Bool) {
            for id in ids {
                VnMissCandidate.top40[id] = isTop40
            }
        }

        pub fun updateCandidate(
            id: UInt64,
            name: String,
            code: String,
            description: String,
            fundAddress: Address,
            properties: {String: String}
        ) {
            pre {
                VnMissCandidate.listCandidate.containsKey(id): "Candidate not exist"
            }
            
            VnMissCandidate.listCandidate[id] = Candidate(id: id, name: name, code: code, description: description, fundAddress: fundAddress, properties: properties)
            emit CandidateUpdate(id: id, name: name, fundAddress: fundAddress)
        }
    }

    pub fun getCandidate(id: UInt64): Candidate? {
        return self.listCandidate[id]
    }

    init() {
        self.listCandidate = {}
        self.AdminStoragePath = /storage/BNVNMissCandidateAdmin
        self.MaxCandidate = 71
        self.top40 = {}

        self.account.save(<- create Admin(), to: self.AdminStoragePath)
    }
}