//Arlee Partner NFT Contract

/*  This contract defines ArleePartner NFTs.
    Users can buy this NFT whenever to enjoy advanced features on Arlequin paint.
    The fund received will not deposit to the Admin wallet, 
    but another wallet that will be shared to the partners.

    Minting the advanced ArleeScenes will require the holders holding the NFT.

    Will be incorporated to Arlee Contract 

    ** The Marketpalce Royalty need to be confirmed.
 */

 import NonFungibleToken from 0x1d7e57aa55817448
 import MetadataViews from 0x1d7e57aa55817448

 pub contract ArleePartner : NonFungibleToken{

    // Total number of ArleePartner NFT in existance
    pub var totalSupply: UInt64 

    // Minted ArleePartner NFT maps with name {ID : Name}
    access(account) var mintedArleePartnerNFTs : {UInt64 : String}

    // Stores all ownedScenes { Owner : Scene IDs }
    access(account) var ownedArleePartnerNFTs : {Address : [UInt64]}

    // Mint Status
    pub var mintable: Bool

    // Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Created(id: UInt64, royalties: [Royalty])

    pub event PartnerRoyaltyAdded(creditor:String, wallet:Address, cut: UFix64)
    pub event PartnerRoyaltyRemoved(creditor:String)
    pub event RoyaltyUpdated(creditor:String, previousCut:UFix64, newCut: UFix64)

    // Paths
    pub let CollectionStoragePath : StoragePath
    pub let CollectionPublicPath : PublicPath

    // Dictionary to stores Partner names whether it is able to mint (i.e. acts as to enable / disable specific ArleePartner NFT minting)
    access(account) var mintableArleePartnerNFTList : {String : Bool}

    // All Royalties (Arlee + Partners Royalty)
    access(account) let allRoyalties: {String : Royalty}

    // Royalty Struct (For later royalty and marketplace implementation)
    pub struct Royalty{
        pub let creditor: String
        pub let wallet: Address
        pub(set) var cut: UFix64

        init(creditor:String, wallet: Address, cut: UFix64){
            self.creditor = creditor
            self.wallet = wallet
            self.cut = cut
        }
    }

    // ArleePartnerNFT (Will only be given name and royalty)
    pub resource NFT : NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64
        pub let name: String
        access(contract) let royalties: [Royalty]

        init(name: String, royalties:[Royalty]){
            self.id = ArleePartner.totalSupply
            self.name = name
            self.royalties = royalties

            // update totalSupply
            ArleePartner.totalSupply = ArleePartner.totalSupply +1
        }

        // Function to return royalty
        pub fun getRoyalties(): [Royalty] {
            return self.royalties
        }

        // MetadataViews Implementation
        pub fun getViews(): [Type] {
          return [Type<MetadataViews.Display>(), 
                  Type<[Royalty]>()]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                    name : self.name ,
                    description : "Holder of the NFT can access advanced features of Arlequin Painter." ,
                    thumbnail : MetadataViews.HTTPFile(url:"https://painter.arlequin.gg/")
                    )

                case Type<[Royalty]>():
                    return self.royalties
            } 
            return nil
        }
    }
    

    // Collection Interfaces Needed for borrowing NFTs
    pub resource interface CollectionPublic {
        pub fun getIDs() : [UInt64]
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(collection: @NonFungibleToken.Collection)
        pub fun borrowNFT(id : UInt64) : &NonFungibleToken.NFT
        pub fun borrowViewResolver(id: UInt64): &{MetadataViews.Resolver}
        pub fun borrowArleePartner(id : UInt64) : &ArleePartner.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Component reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection that implements NonFungible Token Standard with Collection Public and MetaDataViews
    pub resource Collection : CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        pub var ownedNFTs : @{UInt64: NonFungibleToken.NFT}

        init(){
            self.ownedNFTs <- {}
        }

        destroy(){
            destroy self.ownedNFTs

            // remove all IDs owned in the contract upon destruction
            if self.owner != nil {
                ArleePartner.ownedArleePartnerNFTs.remove(key: self.owner!.address)
            }
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot find ArleePartner NFT in your Collection, id: ".concat(withdrawID.toString()))

            emit Withdraw(id: token.id, from: self.owner?.address)

            // update IDs for contract record
            if self.owner != nil {
                ArleePartner.ownedArleePartnerNFTs[self.owner!.address] = self.getIDs()
            }

            return <- token
        }

        pub fun batchWithdraw(withdrawIDs: [UInt64]): @NonFungibleToken.Collection{
            let collection <- ArleePartner.createEmptyCollection()
            for id in withdrawIDs {
                let nft <- self.ownedNFTs.remove(key: id) ?? panic("Cannot find ArleePartner NFT in your Collection, id: ".concat(id.toString()))
                collection.deposit(token: <- nft) 
            }
            return <- collection
        }

        pub fun deposit(token: @NonFungibleToken.NFT){
            let token <- token as! @ArleePartner.NFT

            let id:UInt64 = token.id

            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id:id, to: self.owner?.address)

            // update IDs for contract record
            if self.owner != nil {
                ArleePartner.ownedArleePartnerNFTs[self.owner!.address] = self.getIDs()
            }

            destroy oldToken
        }

        pub fun batchDeposit(collection: @NonFungibleToken.Collection){
            for id in collection.getIDs() {
                let token <- collection.withdraw(withdrawID: id)
                self.deposit(token: <- token)
            }
            destroy collection
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowArleePartner(id: UInt64): &ArleePartner.NFT? {
            if self.ownedNFTs[id] == nil {
                return nil
            }

            let nftRef = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let ref = nftRef as! &ArleePartner.NFT

            return ref
            
        }

        //MetadataViews Implementation
        pub fun borrowViewResolver(id: UInt64): &{MetadataViews.Resolver} {
            let nftRef = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let ArleePartnerRef = nftRef as! &ArleePartner.NFT

            return ArleePartnerRef as &{MetadataViews.Resolver}
        }

    }

    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    /* Query Function (Can also be done in Arlee Contract) */
    // return true if the address holds the ArleePartner NFT
    pub fun checkArleePartnerNFT(addr: Address): Bool {
        let holderCap = getAccount(addr).getCapability<&ArleePartner.Collection{ArleePartner.CollectionPublic}>(ArleePartner.CollectionPublicPath)
        
        if holderCap.borrow == nil {
            return false
        }
        
        let holderRef = holderCap.borrow() ?? panic("Cannot borrow Arlee ArleePartner NFT Reference")
        let ids = holderRef.getIDs()
        if ids.length < 1{
            return false
        }
        return true
    }

    pub fun getArleePartnerNFTIDs(addr: Address): [UInt64]? {
        let holderCap = getAccount(addr).getCapability<&ArleePartner.Collection{ArleePartner.CollectionPublic}>(ArleePartner.CollectionPublicPath)
        
        if holderCap.borrow == nil {
            return nil
        }
        
        let holderRef = holderCap.borrow() ?? panic("Cannot borrow Arlee ArleePartner Collection Reference")
        return holderRef.getIDs()

    }

    pub fun getArleePartnerNFTName(id: UInt64) : String? {
        return ArleePartner.mintedArleePartnerNFTs[id]
    }

    pub fun getArleePartnerNFTNames(addr: Address) : [String]? {
        let ids = ArleePartner.getArleePartnerNFTIDs(addr: addr)
        if ids == nil || ids!.length == 0 {
            return nil
        }

        var list : [String] = []
        for id in ids! {
            let name = ArleePartner.getArleePartnerNFTName(id: id) ?? panic("This id is not mapped to a ArleePartner NFT Name")
            if !list.contains(name) {
                list.append(name)
            }
        }
        return list
    }

    pub fun getAllArleePartnerNFTNames() : {UInt64 : String} {
        return ArleePartner.mintedArleePartnerNFTs
    }

    pub fun getRoyalties(): {String : Royalty} {
        let royalties = ArleePartner.allRoyalties
        return ArleePartner.allRoyalties
    }

    pub fun getPartnerRoyalty(partner: String) : Royalty? {
        for partnerName in ArleePartner.allRoyalties.keys{
            if partnerName == partner{
                return ArleePartner.allRoyalties[partnerName]!
            }
        }
        return nil
    }

    pub fun getOwner(id: UInt64) : Address? {
        for addr in ArleePartner.ownedArleePartnerNFTs.keys{
            if ArleePartner.ownedArleePartnerNFTs[addr]!.contains(id) {
                return addr
            }
        }
        return nil
    }

    pub fun getMintable() : {String : Bool} {
        var mintableDict :  {String : Bool} = {}
        // if mintable is disabled, return all false
        if !ArleePartner.mintable {
            for key in ArleePartner.mintableArleePartnerNFTList.keys{
                mintableDict[key] = false
            }
            return mintableDict
        }

        return ArleePartner.mintableArleePartnerNFTList
    }





    /* Admin Function */
    // Add a new recipient as a partner to receive royalty cut
    access(account) fun addPartner(creditor: String, addr: Address, cut: UFix64 ) {
        // check if this creditor already exist
        assert(!ArleePartner.allRoyalties.containsKey(creditor), message:"This Royalty already exist")  

        let newRoyalty = Royalty(creditor:creditor, wallet: addr, cut: cut)
        // append royalties
        ArleePartner.allRoyalties[creditor] = newRoyalty

        // add the partner name to the mintableArleePartnerNFTList so by default it is mintable.
        ArleePartner.mintableArleePartnerNFTList[creditor] = true

        // emit event
        emit PartnerRoyaltyAdded(creditor:creditor , wallet:addr , cut:cut)
    }

    access(account) fun removePartner(creditor: String ) {
        ArleePartner.allRoyalties.remove(key: creditor) ?? panic("This Royalty does not exist")

        ArleePartner.mintableArleePartnerNFTList.remove(key: creditor) ?? panic("This Partner does not exist")

        emit PartnerRoyaltyRemoved(creditor:creditor)

    }

    access(account) fun setMarketplaceCut(cut: UFix64) {
        let partner = "Arlequin"
        let royaltyRed = &ArleePartner.allRoyalties[partner] as! &Royalty
        let oldRoyalty = royaltyRed.cut
        royaltyRed.cut = cut
        emit RoyaltyUpdated(creditor:"Arlequin", previousCut:oldRoyalty, newCut: cut)
    }

    access(account) fun setPartnerCut(partner: String, cut: UFix64) {
        pre{
            ArleePartner.allRoyalties.containsKey(partner) : "This creditor does not exist"
        }
        let royaltyRed = &ArleePartner.allRoyalties[partner]  as! &Royalty
        let oldRoyalty = royaltyRed.cut
        royaltyRed.cut = cut
        emit RoyaltyUpdated(creditor:partner, previousCut:oldRoyalty, newCut: cut)
    }

    access(account) fun setMintable(mintable: Bool) {
        ArleePartner.mintable = mintable
    }

    access(account) fun setSpecificPartnerNFTMintable(partner: String, mintable: Bool){
        pre{
            ArleePartner.allRoyalties.containsKey(partner) : "This partner does not exist"
        }
        ArleePartner.mintableArleePartnerNFTList[partner] = mintable
    }

    access(account) fun mintArleePartnerNFT(recipient:&{ArleePartner.CollectionPublic}, partner: String) {
        pre{
            ArleePartner.mintable : "Public minting is not available at the moment."
        }

        let overallRoyalties = ArleePartner.getRoyalties()
        let partnerRoyalty = overallRoyalties[partner] ?? panic("Cannot find this partner royalty : ".concat(partner))

        // panic if the specific partner minting is disabled
        assert(ArleePartner.mintableArleePartnerNFTList[partner] != nil, message: "Cannot find this partner : ".concat(partner))
        assert(ArleePartner.mintableArleePartnerNFTList[partner]!, message: "This partner NFT minting is disabled. Partner :".concat(partner))

        let arlequinRoyalty = overallRoyalties["Arlequin"]!
        let newNFT <- create ArleePartner.NFT(name: partner, royalties:[arlequinRoyalty,partnerRoyalty])
        
        ArleePartner.mintedArleePartnerNFTs[newNFT.id] = newNFT.name

        emit Created(id:newNFT.id, royalties:newNFT.getRoyalties())
        recipient.deposit(token: <- newNFT) 
    }

    access(account) fun adminMintArleePartnerNFT(recipient:&{ArleePartner.CollectionPublic}, partner: String) {

        let overallRoyalties = ArleePartner.getRoyalties()
        let partnerRoyalty = overallRoyalties[partner] ?? panic("Cannot find this partner royalty : ".concat(partner))

        // panic if the specific partner is missing, regardless of whether its mintable.
        assert(ArleePartner.mintableArleePartnerNFTList[partner] != nil, message: "Cannot find this partner : ".concat(partner))

        let arlequinRoyalty = overallRoyalties["Arlequin"]!
        let newNFT <- create ArleePartner.NFT(name: partner, royalties:[arlequinRoyalty,partnerRoyalty])
        
        ArleePartner.mintedArleePartnerNFTs[newNFT.id] = newNFT.name

        emit Created(id:newNFT.id, royalties:newNFT.getRoyalties())
        recipient.deposit(token: <- newNFT) 
    }



    init(){
        self.totalSupply = 0

        self.mintableArleePartnerNFTList = {}

        self.mintedArleePartnerNFTs = {}
        self.ownedArleePartnerNFTs = {}

        self.mintable = false

        // Paths
        self.CollectionStoragePath = /storage/ArleePartner
        self.CollectionPublicPath = /public/ArleePartner

        // Royalty
        self.allRoyalties = {"Arlequin" : Royalty(creditor: "Arlequin", wallet: self.account.address, cut: 0.05)}

        // Setup Account
        
        self.account.save(<- ArleePartner.createEmptyCollection() , to: ArleePartner.CollectionStoragePath)
        self.account.link<&ArleePartner.Collection{ArleePartner.CollectionPublic, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection}>(ArleePartner.CollectionPublicPath, target:ArleePartner.CollectionStoragePath)
        
    }
        
 }
 