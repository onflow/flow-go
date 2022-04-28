pub contract Content {

    pub var totalSupply: UInt64

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPrivatePath: PrivatePath

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Created(id: UInt64)

    pub resource Blob {
        pub let id: UInt64

        access(contract) var content: String

        init(initID: UInt64, content: String) {
            self.id = initID
            self.content=content
        }
    }

    //return the content for this NFT
    pub resource interface PublicContent {
        pub fun content(_ id: UInt64): String? 
    }

    pub resource Collection: PublicContent {
        pub var contents: @{UInt64: Blob}

        init () {
            self.contents <- {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @Blob {
            let token <- self.contents.remove(key: withdrawID) ?? panic("missing content")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @Blob) {

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.contents[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.contents.keys
        }

        pub fun content(_ id: UInt64) : String {
            return self.contents[id]?.content ?? panic("Content blob does not exist")
        }

        destroy() {
            destroy self.contents
        }
    }

    access(account) fun createEmptyCollection(): @Content.Collection {
        return <- create Collection()
    }


    access(account) fun createContent(_ content: String) : @Content.Blob {

        var newNFT <- create Blob(initID: Content.totalSupply, content:content)
        emit Created(id: Content.totalSupply)

        Content.totalSupply = Content.totalSupply + UInt64(1)
        return <- newNFT
    }

    init() {
        // Initialize the total supply
        self.totalSupply = 0
        self.CollectionPrivatePath=/private/versusContentCollection
        self.CollectionStoragePath=/storage/versusContentCollection

        let account =self.account
        account.save(<- Content.createEmptyCollection(), to: Content.CollectionStoragePath)
        account.link<&Content.Collection>(Content.CollectionPrivatePath, target: Content.CollectionStoragePath)

        emit ContractInitialized()
    }
}
