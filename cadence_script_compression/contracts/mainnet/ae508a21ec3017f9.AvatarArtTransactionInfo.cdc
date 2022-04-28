import FungibleToken from 0xf233dcee88fe0abe;

pub contract AvatarArtTransactionInfo {
    access(self) var acceptCurrencies: [Type]
    access(self) var nftInfo: {UInt64: NFTInfo}

    pub let FeeInfoStoragePath: StoragePath
    pub let FeeInfoPublicPath: PublicPath

    pub let TransactionAddressStoragePath: StoragePath
    pub let TransactionAddressPublicPath: PublicPath

    pub let AdminStoragePath: StoragePath

    pub event FeeUpdated(tokenId: UInt64, affiliate: UFix64, storing: UFix64, insurance: UFix64, contractor: UFix64, platform: UFix64)
    pub event TransactionAddressUpdated(tokenId: UInt64, storing: Address?, insurance: Address?, contractor: Address?, platform: Address?)
    pub event DefaultAddressChanged(storing: Address?, insurance: Address?, contractor: Address?, platform: Address?)
    pub event DefaultFeeChanged(affiliate: UFix64, storing: UFix64, insurance: UFix64, contractor: UFix64, platform: UFix64)

    pub struct NFTInfo {
        // 0 for Digital, 1 for Physical
        pub let type: UInt
        pub var isFirstOwner: Bool
        pub var author: { String: Capability<&{FungibleToken.Receiver}> } 
        pub var authorFee: UFix64?

        init(type: UInt, authorFee: UFix64?, author: { String: Capability<&{FungibleToken.Receiver}> }) {
            self.type = type
            self.isFirstOwner = true
            self.author = author
            self.authorFee = authorFee
        }

        access(contract) fun setFirstOwner(_ isFirst: Bool) {
            self.isFirstOwner = isFirst
        }
    }
    

    pub struct FeeInfoItem{
        pub let affiliate: UFix64
        pub let storing: UFix64
        pub let insurance: UFix64
        pub let contractor: UFix64
        pub let platform: UFix64

        init (_affiliate: UFix64, _storing: UFix64, _insurance: UFix64, _contractor: UFix64, _platform: UFix64) {
            self.affiliate = _affiliate
            self.storing = _storing
            self.insurance = _insurance
            self.contractor = _contractor
            self.platform = _platform
        }
    }

    pub resource interface PublicFeeInfo{
        pub fun getFee(tokenId: UInt64): FeeInfoItem?
    }

    pub resource FeeInfo : PublicFeeInfo{


        //Store fee for each NFT specific
        pub var fees: {UInt64: FeeInfoItem}

        // The defaults fee if not specific
        // 0: Digital
        // 1: Physical 1st
        // 2: Physical 2st
        pub var defaultFees: {
            UInt: FeeInfoItem
        }

        pub fun setFee(tokenId: UInt64, affiliate: UFix64, storing: UFix64, insurance: UFix64, contractor: UFix64, platform: UFix64){
            pre{
                tokenId > 0: "tokenId parameter is zero"
            }

            self.fees[tokenId] = FeeInfoItem(
                _affiliate: affiliate,
                _storing: storing,
                _insurance: insurance,
                _contractor: contractor,
                _platform: platform)

            emit FeeUpdated(tokenId: tokenId, affiliate: affiliate, storing: storing, insurance: insurance, contractor: contractor, platform: platform)
        }

        pub fun getFee(tokenId: UInt64): FeeInfoItem?{
            pre{
                tokenId > 0: "tokenId parameter is zero"
            }

            let nftInfo = AvatarArtTransactionInfo.nftInfo[tokenId]
            if nftInfo == nil {
                return nil
            }

            // For Digital NFT
            if nftInfo!.type == 0 {
                return self.defaultFees[0]
            }

            // For Physical NFT
            if nftInfo!.type == 1 {
                if nftInfo!.isFirstOwner {
                    return  self.fees[tokenId] ?? self.defaultFees[1]
                }

                return  self.defaultFees[2]
            }

            return nil
        }

        pub fun setDefaultFee(type: UInt, affiliate: UFix64, storing: UFix64, insurance: UFix64, contractor: UFix64, platform: UFix64) {
            pre {
                [(0 as UInt), 1, 2].contains(type): "Type should be 0, 1, 2"
            }

            self.defaultFees[type] = FeeInfoItem(
                _affiliate: affiliate,
                _storing: storing,
                _insurance: insurance,
                _contractor: contractor,
                _platform: platform)

            emit DefaultFeeChanged(affiliate: affiliate, storing: storing, insurance: insurance, contractor: contractor, platform: platform)
        }

        // initializer
        init () {
            self.fees = {}
            self.defaultFees = {}
        }

        // destructor
        destroy() {
            //Do nothing
        }
    }
    
    pub struct TransactionRecipientItem{
        pub let storing: Capability<&{FungibleToken.Receiver}>?
        pub let insurance: Capability<&{FungibleToken.Receiver}>?
        pub let contractor: Capability<&{FungibleToken.Receiver}>?
        pub let platform: Capability<&{FungibleToken.Receiver}>?

        init (_storing: Capability<&{FungibleToken.Receiver}>?, 
            _insurance: Capability<&{FungibleToken.Receiver}>?, 
            _contractor: Capability<&{FungibleToken.Receiver}>?, 
            _platform: Capability<&{FungibleToken.Receiver}>?) {
            self.storing = _storing
            self.insurance = _insurance
            self.contractor = _contractor
            self.platform = _platform
        }
    }

    pub resource interface PublicTransactionAddress{
        pub fun getAddress(tokenId: UInt64, payType: Type): TransactionRecipientItem?
    }

    pub resource TransactionAddress : PublicTransactionAddress{
        // Store fee for each NFT
        // map tokenID => { payTypeIdentifier => TransactionRecipientItem }
        pub var addresses: {UInt64: {String: TransactionRecipientItem}}
        pub var defaultAddresses:  {String: TransactionRecipientItem}

        pub fun setAddress(tokenId: UInt64, payType: Type,
            storing: Capability<&{FungibleToken.Receiver}>?, 
            insurance: Capability<&{FungibleToken.Receiver}>?, 
            contractor: Capability<&{FungibleToken.Receiver}>?, 
            platform: Capability<&{FungibleToken.Receiver}>?){
            pre {
                tokenId > 0: "tokenId parameter is zero"
            }

            let address = self.addresses[tokenId] ?? {}

            address.insert(
                key: payType.identifier,
                TransactionRecipientItem(
                    _storing: storing,
                    _insurance: insurance,
                    _contractor: contractor,
                    _platform: platform)
            )

            self.addresses[tokenId] = address

            emit TransactionAddressUpdated(
                tokenId: tokenId,
                storing: storing?.address,
                insurance: insurance?.address,
                contractor: contractor?.address,
                platform: platform?.address
            )
        }

        pub fun getAddress(tokenId: UInt64, payType: Type): TransactionRecipientItem?{
            pre {
                tokenId > 0: "tokenId parameter is zero"
            }

            if let addr = self.addresses[tokenId] {
                return addr[payType.identifier]
            }
            return self.defaultAddresses[payType.identifier]
        }

        pub fun setDefaultAddress(payType: Type,
            storing: Capability<&{FungibleToken.Receiver}>?, 
            insurance: Capability<&{FungibleToken.Receiver}>?, 
            contractor: Capability<&{FungibleToken.Receiver}>?, 
            platform: Capability<&{FungibleToken.Receiver}>?){

            self.defaultAddresses.insert(
                key: payType.identifier,
                TransactionRecipientItem(
                    _storing: storing,
                    _insurance: insurance,
                    _contractor: contractor,
                    _platform: platform
                )
            )

            emit DefaultAddressChanged(
                storing: storing?.address,
                insurance: insurance?.address,
                contractor: contractor?.address,
                platform: platform?.address
            )
        }

        init () {
            self.addresses = {}
            self.defaultAddresses = {}
        }

        // destructor
        destroy() {
            //Do nothing
        }
    }

    pub resource Administrator {
        pub fun setAcceptCurrencies(types: [Type]) {
            for type in types {
                assert(
                type.isSubtype(of: Type<@FungibleToken.Vault>()),
                message: "Should be a sub type of FungibleToken.Vault"
                )
            }

            AvatarArtTransactionInfo.acceptCurrencies = types
        }

        pub fun setNFTInfo(tokenID: UInt64, type: UInt, author: {String: Capability<&{FungibleToken.Receiver}>}, authorFee: UFix64?) {
            AvatarArtTransactionInfo.nftInfo[tokenID] = NFTInfo(type: type,  authorFee: authorFee, author: author)
        }
    }

    access(account) fun setFirstOwner(tokenID: UInt64, _ isFirst: Bool) {
        if let nft = self.nftInfo[tokenID] {
            nft.setFirstOwner(isFirst)
            self.nftInfo[tokenID] = nft
        }
    }

    access(account) fun getNFTInfo(tokenID: UInt64): NFTInfo? {
       return  self.nftInfo[tokenID] 
    }

    pub fun getAcceptCurrentcies(): [Type] {
        return  self.acceptCurrencies
    }

    pub fun isCurrencyAccepted(type: Type): Bool {
        return  self.acceptCurrencies.contains(type)
    }

    init(){
        self.acceptCurrencies = []
        self.nftInfo = {}
        self.FeeInfoStoragePath = /storage/avatarArtTransactionInfoFeeInfo
        self.FeeInfoPublicPath = /public/avatarArtTransactionInfoFeeInfo

        self.TransactionAddressStoragePath = /storage/avatarArtTransactionInfoRecepientAddress
        self.TransactionAddressPublicPath = /public/avatarArtTransactionInfoRecepientAddress

        let feeInfo <- create FeeInfo()
        self.account.save(<- feeInfo, to: self.FeeInfoStoragePath)

        self.account.link<&AvatarArtTransactionInfo.FeeInfo{AvatarArtTransactionInfo.PublicFeeInfo}>(
            AvatarArtTransactionInfo.FeeInfoPublicPath,
            target: AvatarArtTransactionInfo.FeeInfoStoragePath)

        let transactionAddress <- create TransactionAddress()
        self.account.save(<- transactionAddress, to: self.TransactionAddressStoragePath)

        self.account.link<&AvatarArtTransactionInfo.TransactionAddress{AvatarArtTransactionInfo.PublicTransactionAddress}>(
            AvatarArtTransactionInfo.TransactionAddressPublicPath,
            target: AvatarArtTransactionInfo.TransactionAddressStoragePath)

        self.AdminStoragePath = /storage/avatarArtTransactionInfoAdmin
        self.account.save(<- create Administrator(), to: self.AdminStoragePath)
    }
}