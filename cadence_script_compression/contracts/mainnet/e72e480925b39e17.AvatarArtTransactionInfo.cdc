import FungibleToken from 0xf233dcee88fe0abe


pub contract AvatarArtTransactionInfo {
    access(self) var acceptCurrencies: [Type];

    pub let FeeInfoStoragePath: StoragePath;
    pub let FeeInfoPublicPath: PublicPath;

    pub let TransactionAddressStoragePath: StoragePath;
    pub let TransactionAddressPublicPath: PublicPath;

    pub let AdminStoragePath: StoragePath;

    pub event FeeUpdated(tokenId: UInt64, affiliate: UFix64, storing: UFix64, insurance: UFix64, contractor: UFix64, platform: UFix64, author: UFix64);
    pub event TransactionAddressUpdated(tokenId: UInt64, storing: Address?, insurance: Address?, contractor: Address?, platform: Address?, author: Address?);
    

    pub struct FeeInfoItem{
        pub let affiliate: UFix64;
        pub let storing: UFix64;
        pub let insurance: UFix64;
        pub let contractor: UFix64;
        pub let platform: UFix64;
        pub let author: UFix64;

        // initializer
        init (_affiliate: UFix64, _storing: UFix64, _insurance: UFix64, _contractor: UFix64, _platform: UFix64, _author: UFix64) {
            self.affiliate = _affiliate;
            self.storing = _storing;
            self.insurance = _insurance;
            self.contractor = _contractor;
            self.platform = _platform;
            self.author = _author;
        }
    }

    pub resource interface PublicFeeInfo{
        pub fun getFee(tokenId: UInt64): FeeInfoItem?;
    }

    pub resource FeeInfo : PublicFeeInfo{
        //Store fee for each NFT
        pub var fees: {UInt64: FeeInfoItem};

        pub fun setFee(tokenId: UInt64, affiliate: UFix64, storing: UFix64, insurance: UFix64, contractor: UFix64, platform: UFix64, author: UFix64){
            pre{
                tokenId > 0: "tokenId parameter is zero";
            }

            self.fees[tokenId] = FeeInfoItem(
                _affiliate: affiliate,
                _storing: storing,
                _insurance: insurance,
                _contractor: contractor,
                _platform: platform,
                _author: author);

            emit FeeUpdated(tokenId: tokenId, affiliate: affiliate, storing: storing, insurance: insurance, contractor: contractor, platform: platform, author: author);
        }

        pub fun getFee(tokenId: UInt64): FeeInfoItem?{
            pre{
                tokenId > 0: "tokenId parameter is zero";
            }

            return self.fees[tokenId];
        }

        // initializer
        init () {
            self.fees = {};
        }

        // destructor
        destroy() {
            //Do nothing
        }
    }
    
    pub struct TransactionRecipientItem{
        pub let storing: Capability<&{FungibleToken.Receiver}>?;
        pub let insurance: Capability<&{FungibleToken.Receiver}>?;
        pub let contractor: Capability<&{FungibleToken.Receiver}>?;
        pub let platform: Capability<&{FungibleToken.Receiver}>?;
        pub let author: Capability<&{FungibleToken.Receiver}>?;

        // initializer
        init (_storing: Capability<&{FungibleToken.Receiver}>?, 
            _insurance: Capability<&{FungibleToken.Receiver}>?, 
            _contractor: Capability<&{FungibleToken.Receiver}>?, 
            _platform: Capability<&{FungibleToken.Receiver}>?, 
            _author: Capability<&{FungibleToken.Receiver}>?) {
            self.storing = _storing;
            self.insurance = _insurance;
            self.contractor = _contractor;
            self.platform = _platform;
            self.author = _author;
        }
    }

    pub resource interface PublicTransactionAddress{
        pub fun getAddress(tokenId: UInt64, payType: Type): TransactionRecipientItem?;
    }

    pub resource TransactionAddress : PublicTransactionAddress{
        //Store fee for each NFT
        // map tokenID => { payTypeIdentifier => TransactionRecipientItem }
        pub var addresses: {UInt64: {String: TransactionRecipientItem}};

        pub fun setAddress(tokenId: UInt64, payType: Type,
            storing: Capability<&{FungibleToken.Receiver}>?, 
            insurance: Capability<&{FungibleToken.Receiver}>?, 
            contractor: Capability<&{FungibleToken.Receiver}>?, 
            platform: Capability<&{FungibleToken.Receiver}>?, 
            author: Capability<&{FungibleToken.Receiver}>?){
            pre {
                tokenId > 0: "tokenId parameter is zero";
            }

            let address = self.addresses[tokenId] ?? {};

            address.insert(
                key: payType.identifier,
                TransactionRecipientItem(
                    _storing: storing,
                    _insurance: insurance,
                    _contractor: contractor,
                    _platform: platform,
                    _author: author)
            )

            self.addresses[tokenId] = address;

            emit TransactionAddressUpdated(
                tokenId: tokenId,
                storing: storing?.address,
                insurance: insurance?.address,
                contractor: contractor?.address,
                platform: platform?.address,
                author: author?.address
            );
        }

        pub fun getAddress(tokenId: UInt64, payType: Type): TransactionRecipientItem?{
            pre {
                tokenId > 0: "tokenId parameter is zero";
            }

            if let addr = self.addresses[tokenId] {
                return addr[payType.identifier]
            }
            return nil
        }

        // initializer
        init () {
            self.addresses = {};
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

            AvatarArtTransactionInfo.acceptCurrencies = types;
        }
    }

    pub fun getAcceptCurrentcies(): [Type] {
        return  self.acceptCurrencies;
    }

    pub fun isCurrencyAccepted(type: Type): Bool {
        return  self.acceptCurrencies.contains(type);
    }

    init(){
        self.acceptCurrencies = [];
        self.FeeInfoStoragePath = /storage/avatarArtTransactionInfoFeeInfo04;
        self.FeeInfoPublicPath = /public/avatarArtTransactionInfoFeeInfo04;

        self.TransactionAddressStoragePath = /storage/avatarArtTransactionInfoRecepientAddress04;
        self.TransactionAddressPublicPath = /public/avatarArtTransactionInfoRecepientAddress04;

        let feeInfo <- create FeeInfo();
        self.account.save(<- feeInfo, to: self.FeeInfoStoragePath);

        self.account.link<&AvatarArtTransactionInfo.FeeInfo{AvatarArtTransactionInfo.PublicFeeInfo}>(
            AvatarArtTransactionInfo.FeeInfoPublicPath,
            target: AvatarArtTransactionInfo.FeeInfoStoragePath);

        let transactionAddress <- create TransactionAddress();
        self.account.save(<- transactionAddress, to: self.TransactionAddressStoragePath);

        self.account.link<&AvatarArtTransactionInfo.TransactionAddress{AvatarArtTransactionInfo.PublicTransactionAddress}>(
            AvatarArtTransactionInfo.TransactionAddressPublicPath,
            target: AvatarArtTransactionInfo.TransactionAddressStoragePath);

        self.AdminStoragePath = /storage/avatarArtTransactionInfoAdmin04
        self.account.save(<- create Administrator(), to: self.AdminStoragePath);
    }
}
