import NonFungibleToken from 0x1d7e57aa55817448
import NyatheesOVO from 0x75e0b6de94eb05d0

// import NonFungibleToken from 0xacf3dfa413e00f9f
// import NyatheesOVO from 0xacf3dfa413e00f9f

import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393

pub contract MysteryBox {

    /// @notice
    /// @dev event of open mystery box
    /// @param orderId, created when user buy mystery box
    /// @param buyer, user who buy mystery box
    /// @param type, which mystery box user selected
    /// @param unitPrice, unit price of a mystery box
    /// @return
    pub event OpenMysteryBox(orderId: UInt64,
                             buyer: Address,
                             type: UInt64,
                             unitPrice: UFix64,
                             tokenIds: [UInt64],
                             poolTokenName: String,
                             amountAddToPool: UFix64)

    /// @notice
    /// @dev event of rebate to users
    /// @param orderId, created when user buy mystery box
    /// @param token, using which FT to buy mystery box
    /// @param users, who rebate to
    /// @param amounts, user address => promotion profit amount
    /// @return
    pub event RebateToUser(token: String, user: Address, amount: UFix64)

    /// @notice
    /// @dev event of invitation
    /// @param user, the new user
    /// @param referrer, as it means
    /// @return
    pub event Invitation(user: Address, referrer: Address)

    /// @notice
    /// @dev event of update prize pool
    /// @param tokenName, just the name of a token, as a key in dictionary
    /// @param amount, as it means
    /// @return
    pub event UpdatePrizePool(tokenName: String, amount: UFix64)

    /// @notice
    /// @dev event of PrizePoolReward
    /// @param round, 1 week as a round
    /// @param addresses, users who will get the reward
    /// @param amounts, amount of users who will get how much token in addresses
    /// @param total, total amount of tokens will give to users
    /// @return
    pub event PrizePoolReward(round: UInt64, addresses: [Address], amounts: [UFix64], total: UFix64)

    // event of nft provider resources destroyed
    pub event nftProviderResourcesDestroyed()

    //path
    pub let MysteryBoxControllerPublicPath: PublicPath
    pub let MysteryBoxControllerPrivatePath: PrivatePath
    pub let MysteryBoxControllerStoragePath: StoragePath

    //const
    pub let ZERO_ADDRESS: Address
    pub let maxMysteryBoxToBuy: UInt64

    //account details
    pub struct accountItem{
        pub let referrerAddr: Address
        pub var active: Bool
        pub var inviteAmount: UInt64

        //mapping token address => balance
        //designed to support multiple tokens
        //so we use mapping to recored
        //how much you promote and earned
        pub var promotes:{String: UFix64}
        pub var earned:{String: UFix64}


        init(referrerAddr: Address, active: Bool, inviteAmount: UInt64, promotes: {String: UFix64}, earned: {String: UFix64}){
            self.referrerAddr = referrerAddr
            self.active = active
            self.inviteAmount = inviteAmount
            self.promotes = promotes
            self.earned = earned
        }
        // access(contract) fun setRefer(address: Address)
    }

    //designed to support mutiple set of mystory boxs
    //using this structure to recored type of the set
    pub struct MysteryBoxType{
        pub let typeId: UInt64
        pub let describe: String
        //amount of NFTs in this type
        pub var stock: UInt64
        //unit price of a mystery box of this type
        pub var unitPrice: UFix64

        init(typeId: UInt64, describe: String, stock: UInt64, unitPrice: UFix64){
            self.typeId = typeId
            self.describe = describe
            self.stock = stock
            self.unitPrice = unitPrice
        }
    }

    //designed to support mutiple tokens prize pool
    //so mapping token address => balance
    access(self) var prizePool: {String: UFix64}
    //account list
    //mapping user address => accountItem
    access(self) var accountList:{Address: accountItem}
    //mystery box type list
    //mapping type id => mysteryBoxTypeItem
    access(self) var mysteryBoxTypeList:{UInt64: MysteryBoxType}
    //how many mystery box were sold
    access(self) var totalSupply: UInt64                                                                                                                                                                                                                                 
    
    pub resource interface MysteryBoxControllerPrivate{

        // functions for admin account
        // set Mystery Box types and stocks
        pub fun setMysteryBoxTypeToList(typeId: UInt64,
                                        describe: String,
                                        stock: UInt64,
                                        unitPrice: UFix64)
        // set prize pool
        pub fun setPrizePool(tokenName: String, amount: UFix64)
        // add totalsupply of mystery box
        pub fun addTotalSupply(amount: UInt64)
        // set NFT provider of Mystery Box, all NTFs in mystery Box
        // are from here
        pub fun setMysteryBoxNFTPrividerCap(prividerCap: Capability<&NyatheesOVO.NFTMinter>)
        // fusd receiver of admin
        pub fun setMainFusdReceiver(mainFusdReceiverAddr: Address)
        // send commission to referrer and referrer`s referrer
        // return the buyerTokenVault, which will be deposited to fusd receiver of admin
        pub fun sendCommission(buyerAddr: Address,
                               buyerTokenVault: @FungibleToken.Vault,
                               tokenName: String,
                               totalPrice: UFix64): @FungibleToken.Vault
        // check invite relationship between users
        pub fun checkInviteRelationAndSaveIt(buyerAddr: Address, referrerAddr: Address)
        // send NFTs to the buyer, which represent mystery boxs were opened
        // return tokenIds of the NFTs which send to the buyer
        pub fun checkStockAndSendNFTToBuyer(buyerAddr: Address,
                                            mysteryBoxTypeId: UInt64,
                                            mysteryBoxAmount: UInt64):[UInt64]
        // send reward to the top 50 of commission receivers
        // addresses.length must be equal to amounts.length
        // addresses[index] => user address
        // amounts[index] => the reward that the user in addresses will get
        //e.g.
        //my address is 0x01, addresses[0] = 0x01
        //my reward is 7FUSD, amounts[0] = 7
        pub fun awardToTop50(round: UInt64,
                             addresses: [Address],
                             tokenName: String,
                             amounts: [UFix64],
                             total: UFix64,
                             paymentVault: @FungibleToken.Vault,
                             payer: Address)
    }

    pub resource interface MysteryBoxControllerPublic{
        //public functions to all users
        //buy mystery box
        //this function will call checkInviteRelationAndSaveIt to check invitation relationship
        // and call sendCommission to send commission to the referrer and referrer`s referrer
        // and call checkStockAndSendNFTToBuyer to send NFTs to buyer
        // then deposit the fusd which user pay
        pub fun buyMysteryBox(mysteryBoxTypeId: UInt64,
                              buyerAddr: Address,
                              referrerAddr: Address,
                              tokenName: String,
                              mysteryBoxAmount: UInt64,
                              buyerTokenVault: @FungibleToken.Vault)
        //return how much mystery box were sold now
        pub fun getTotalSupply():UInt64
        //get account detail info
        pub fun getAccountItem(address: Address): MysteryBox.accountItem?
        pub fun getPrizePoolBalance(tokenName: String): UFix64
        pub fun getMysteryBoxTypeItem(typeId: UInt64): MysteryBox.MysteryBoxType?
    }

    pub resource MysteryBoxController: MysteryBoxControllerPrivate, MysteryBoxControllerPublic{

        // //capability to transfer NFT to users when they buy mystery box
        // access(contract) var nftProviderResources: @[NFTProvider]
        // //index of nftProviderResources which will be used as the NFT provider
        // access(contract) var nftProviderResourcesCurrentIndex: UInt64

        
        //address to receive fusd when user pay for mystery box
        access(self) var mainFusdReceiverAddr: Address
        //capability to transfer NFT to users when they buy mystery box
        access(self) var nftProviderCap: Capability<&NyatheesOVO.NFTMinter>?

        pub fun buyMysteryBox(mysteryBoxTypeId: UInt64,
                              buyerAddr: Address,
                              referrerAddr: Address,
                              tokenName: String,
                              mysteryBoxAmount: UInt64,
                              buyerTokenVault: @FungibleToken.Vault){
            pre {
                mysteryBoxAmount > 0 && mysteryBoxAmount <= MysteryBox.maxMysteryBoxToBuy : "wrong mystery box amount"
                tokenName != nil && tokenName != "" : "wrong token name"
                buyerAddr != nil && buyerAddr != MysteryBox.ZERO_ADDRESS : "wrong buyer address"
                referrerAddr != buyerAddr : "you can not use your own address as a referrer"
                buyerTokenVault != nil : "wrong buyer vault"
                MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId] != nil : "wrong mysterybox type"
            }
            // calculate totalPrice, if user buy 10 mysterybox, 90% discount
            var totalPrice = MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId]!.unitPrice * UFix64(mysteryBoxAmount)
            if(mysteryBoxAmount == MysteryBox.maxMysteryBoxToBuy){
                totalPrice = MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId]!.unitPrice * UFix64(mysteryBoxAmount) * 90000000.0 / 100000000.0
            }

            assert(totalPrice >= buyerTokenVault.balance, message: "not enough balance to buy mystery box")

            // check the buyer account info, if not exist, create a new one
            if(!MysteryBox.accountList.containsKey(buyerAddr)){
                MysteryBox.accountList[buyerAddr] = MysteryBox.accountItem(referrerAddr: MysteryBox.ZERO_ADDRESS,
                                                                           active: false,
                                                                           inviteAmount: 0 as UInt64,
                                                                           promotes: {},
                                                                           earned: {})
            }

            self.checkInviteRelationAndSaveIt(buyerAddr:buyerAddr, referrerAddr:referrerAddr)
            var remainderVault <- self.sendCommission(buyerAddr: buyerAddr,
                                                      buyerTokenVault: <-buyerTokenVault,
                                                      tokenName: tokenName,
                                                      totalPrice: totalPrice)
            
            var saledTokenIds = self.checkStockAndSendNFTToBuyer(buyerAddr: buyerAddr,
                                                                 mysteryBoxTypeId: mysteryBoxTypeId,
                                                                 mysteryBoxAmount: mysteryBoxAmount)
            //deposit buyerTokenValut to admin fusd receiver
            var mainFusdReceiverCap = getAccount(self.mainFusdReceiverAddr)
                    .getCapability(/public/fusdReceiver)
                    .borrow<&{FungibleToken.Receiver}>()
                    ?? panic("Unable to borrow receiver reference")
            mainFusdReceiverCap.deposit(from: <- remainderVault)
            emit OpenMysteryBox(orderId: MysteryBox.totalSupply,
                                buyer: buyerAddr,
                                type: mysteryBoxTypeId,
                                unitPrice: MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId]!.unitPrice,
                                tokenIds: saledTokenIds,
                                poolTokenName: tokenName,
                                amountAddToPool: totalPrice * 50000000.0 / 1000000000.0)
            // emit UpdatePrizePool(tokenName: tokenName, amount: totalPrice * 50000000.0 / 1000000000.0)

        }

        pub fun sendCommission(buyerAddr: Address,
                               buyerTokenVault: @FungibleToken.Vault,
                               tokenName: String,
                               totalPrice: UFix64): @FungibleToken.Vault{
            pre {
                MysteryBox.accountList.containsKey(buyerAddr): "User not exist"
                totalPrice > 0.0 : "wrong total price"
            }

            //get referrer address
            let referrerAddr = MysteryBox.accountList[buyerAddr]!.referrerAddr

            //if referrer != self.ZERO_ADDRESS, it means that the user has a referrer
            if (referrerAddr != MysteryBox.ZERO_ADDRESS){
                //send the commission to the referrer
                var referrerCap = getAccount(referrerAddr)
                    .getCapability(/public/fusdReceiver)
                    .borrow<&{FungibleToken.Receiver}>()
                    ?? panic("Unable to borrow receiver fusd reference")

                // referrer get commission of 20% of total price
                var refTempVault <- buyerTokenVault.withdraw(amount:totalPrice * 20000000.0 / 100000000.0)
                referrerCap.deposit(from: <-refTempVault)
                emit RebateToUser(token: tokenName, user: referrerAddr, amount: totalPrice * 20000000.0 / 100000000.0)

                // check the referrer exist or not
                // if referrer exist, record the promote amount to the referrer
                // then check referrer`s referrer and do the same thing
                if (MysteryBox.accountList.containsKey(referrerAddr)){
                    //record promote data and earned data
                    var tempPromotes = MysteryBox.accountList[referrerAddr]!.promotes
                    if (tempPromotes[tokenName] != nil){
                        var promote = tempPromotes[tokenName]!
                        tempPromotes[tokenName] = promote + totalPrice
                    } else {
                        tempPromotes[tokenName] = totalPrice
                    }

                    var tempEarned = MysteryBox.accountList[referrerAddr]!.earned
                    if (tempEarned[tokenName] != nil){
                        var earned = tempEarned[tokenName]!
                        tempEarned[tokenName] = earned + totalPrice * 20000000.0 / 100000000.0
                    } else {
                        tempEarned[tokenName] = totalPrice * 20000000.0 / 100000000.0
                    }
                    MysteryBox.accountList[referrerAddr] = MysteryBox.accountItem(referrerAddr: MysteryBox.accountList[referrerAddr]!.referrerAddr,
                                                                            active: MysteryBox.accountList[referrerAddr]!.active,
                                                                            inviteAmount: MysteryBox.accountList[referrerAddr]!.inviteAmount,
                                                                            promotes: tempPromotes,
                                                                            earned: tempEarned)

                    //check the referrer`s referrer exist or not
                    let lreferrerAddr = MysteryBox.accountList[referrerAddr]!.referrerAddr
                    //send the commission to the referrer`s referrer if it exist
                    if(lreferrerAddr != MysteryBox.ZERO_ADDRESS){
                        var lreferrerCap = getAccount(lreferrerAddr)
                        .getCapability(/public/fusdReceiver)
                        .borrow<&{FungibleToken.Receiver}>()
                        ?? panic("Unable to borrow receiver reference")

                        // referrer`s referrer get commission of 5% of total price
                        var lrefTempVault <- buyerTokenVault.withdraw(amount:totalPrice * 50000000.0 / 1000000000.0)
                        lreferrerCap.deposit(from: <-lrefTempVault)
                        emit RebateToUser(token: tokenName, user: lreferrerAddr, amount: totalPrice * 50000000.0 / 1000000000.0)

                        // check the referrer`s referrer exist or not
                        // if referrer`s referrer exist, record the promote amount to the referrer`s referrer
                        if (MysteryBox.accountList.containsKey(lreferrerAddr)){
                            //record promote data and earned data
                            var tempPromotes = MysteryBox.accountList[lreferrerAddr]!.promotes
                            if (tempPromotes[tokenName] != nil){
                                var promote = tempPromotes[tokenName]!
                                tempPromotes[tokenName] = promote + totalPrice
                            } else {
                                tempPromotes[tokenName] = totalPrice
                            }
                            var tempEarned = MysteryBox.accountList[lreferrerAddr]!.earned
                            if (tempEarned[tokenName] != nil){
                                var earned = tempEarned[tokenName]!
                                tempEarned[tokenName] = earned + totalPrice * 50000000.0 / 1000000000.0
                            } else {
                                tempEarned[tokenName] = totalPrice * 50000000.0 / 1000000000.0
                            }
                            MysteryBox.accountList[lreferrerAddr] = MysteryBox.accountItem(referrerAddr: MysteryBox.accountList[lreferrerAddr]!.referrerAddr,
                                                                                    active: MysteryBox.accountList[lreferrerAddr]!.active,
                                                                                    inviteAmount: MysteryBox.accountList[lreferrerAddr]!.inviteAmount,
                                                                                    promotes: tempPromotes,
                                                                                    earned: tempEarned)
                        }
                    }
                }
            }

            if (MysteryBox.prizePool[tokenName] != nil){
                var tempPoolBalance = MysteryBox.prizePool[tokenName]!
                MysteryBox.prizePool[tokenName] = tempPoolBalance + totalPrice * 50000000.0 / 1000000000.0
            } else {
                MysteryBox.prizePool[tokenName] = totalPrice * 50000000.0 / 1000000000.0
            }

            return <-buyerTokenVault
        }

        pub fun checkInviteRelationAndSaveIt(buyerAddr: Address, referrerAddr: Address){
                pre{
                    MysteryBox.accountList.containsKey(buyerAddr) : "buyer account not exist"
                }

                var buyerAccount = MysteryBox.accountList[buyerAddr]
                // if the buyer has no referrer, save it
                if(buyerAccount!.referrerAddr == MysteryBox.ZERO_ADDRESS && referrerAddr != MysteryBox.ZERO_ADDRESS){
                    MysteryBox.accountList[buyerAddr] = MysteryBox.accountItem(referrerAddr: referrerAddr,
                                                                               active: MysteryBox.accountList[buyerAddr]!.active,
                                                                               inviteAmount: MysteryBox.accountList[buyerAddr]!.inviteAmount,
                                                                               promotes: MysteryBox.accountList[buyerAddr]!.promotes,
                                                                               earned: MysteryBox.accountList[buyerAddr]!.earned)
                    // add inviteAmount of the referrer and referrer`s referrer if they exist
                    if (MysteryBox.accountList.containsKey(referrerAddr)){
                        MysteryBox.accountList[referrerAddr] = MysteryBox.accountItem(referrerAddr: MysteryBox.accountList[referrerAddr]!.referrerAddr,
                                                                                      active: MysteryBox.accountList[referrerAddr]!.active,
                                                                                      inviteAmount: MysteryBox.accountList[referrerAddr]!.inviteAmount + 1,
                                                                                      promotes: MysteryBox.accountList[referrerAddr]!.promotes,
                                                                                      earned: MysteryBox.accountList[referrerAddr]!.earned)
                        emit Invitation(user: buyerAddr, referrer: referrerAddr)
                        // check the referrer`s referrer exist or not
                        // if exist, add inviteAmount of the referrer`s referrer
                        var lreferrerAddr = MysteryBox.accountList[referrerAddr]!.referrerAddr
                        if (lreferrerAddr != nil && lreferrerAddr != MysteryBox.ZERO_ADDRESS && MysteryBox.accountList.containsKey(lreferrerAddr)){
                            MysteryBox.accountList[lreferrerAddr] = MysteryBox.accountItem(referrerAddr: MysteryBox.accountList[lreferrerAddr]!.referrerAddr,
                                                                                           active: MysteryBox.accountList[lreferrerAddr]!.active,
                                                                                           inviteAmount: MysteryBox.accountList[lreferrerAddr]!.inviteAmount + 1,
                                                                                           promotes: MysteryBox.accountList[lreferrerAddr]!.promotes,
                                                                                           earned: MysteryBox.accountList[lreferrerAddr]!.earned)
                        }
                    }              
                }
        }

        pub fun checkStockAndSendNFTToBuyer(buyerAddr: Address, mysteryBoxTypeId: UInt64, mysteryBoxAmount: UInt64):[UInt64]{
            pre{
                mysteryBoxAmount > 0 && mysteryBoxAmount <= MysteryBox.maxMysteryBoxToBuy : "wrong mystery box amount"
                buyerAddr != nil && buyerAddr != MysteryBox.ZERO_ADDRESS : "wrong buyer address"
                mysteryBoxTypeId > 0 : "wrong mystery box type"
                MysteryBox.mysteryBoxTypeList.containsKey(mysteryBoxTypeId) : "mystery box not exist"
                MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId]!.stock >= mysteryBoxAmount : "mystery box not enough"
            }

            var mysteryBoxTypeItem = MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId]
            var stock = MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId]!.stock

            var receiverCap = getAccount(buyerAddr).getCapability(NyatheesOVO.CollectionPublicPath)
						.borrow<&{NonFungibleToken.CollectionPublic}>()?? panic("Unable to borrow NyatheesOVO Receiver!")

            //send NFTs to the buyer
            var saledTokenIds: [UInt64] = []
            var tempMysteryBoxAmount = mysteryBoxAmount
            var tempNftMintCap = self.nftProviderCap!.borrow()!
            while tempMysteryBoxAmount > 0 {
                tempNftMintCap.mintNFTForMysterBox(receiver: receiverCap, metadata: {"url":"https://www.ovo.space/api/v1/metadata/get-metadata?tokenId="})
                saledTokenIds.append(NyatheesOVO.totalSupply)
                stock = stock - 1
                tempMysteryBoxAmount = tempMysteryBoxAmount - 1
            }
            MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId] = MysteryBox.MysteryBoxType(typeId: MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId]!.typeId,
                                                                                        describe: MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId]!.describe,
                                                                                        stock: stock,
                                                                                        unitPrice: MysteryBox.mysteryBoxTypeList[mysteryBoxTypeId]!.unitPrice)
            MysteryBox.totalSupply = MysteryBox.totalSupply + mysteryBoxAmount
            return saledTokenIds
        }
        

        pub fun awardToTop50(round: UInt64,
                             addresses: [Address],
                             tokenName: String,
                             amounts: [UFix64],
                             total: UFix64,
                             paymentVault: @FungibleToken.Vault,
                             payer: Address){
            pre{
                MysteryBox.prizePool[tokenName]! >= total : "pool balance not enough"
                addresses.length == amounts.length : "length of addresses not equal to length of amounts"
                addresses.length > 0 && amounts.length > 0 : "length of addresses or amounts < 0"
            }
            var count = addresses.length
            if (paymentVault.balance >= total) {
                var index = 0
                while count > index{
                    var receiverCap = getAccount(addresses[index])
                        .getCapability(/public/fusdReceiver)
                        .borrow<&{FungibleToken.Receiver}>()
                        ?? panic("Unable to borrow receiver reference")
                    receiverCap.deposit(from: <-paymentVault.withdraw(amount: amounts[index]))
                    index = index + 1
                }

                emit PrizePoolReward(round: round, addresses: addresses, amounts: amounts, total: total)
                var tempPoolBalance = MysteryBox.prizePool[tokenName]!
                MysteryBox.prizePool[tokenName] = tempPoolBalance - total
            }
            var payerCap = getAccount(payer)
                        .getCapability(/public/fusdReceiver)
                        .borrow<&{FungibleToken.Receiver}>()
                        ?? panic("Unable to borrow receiver reference")

            payerCap.deposit(from: <- paymentVault)
        }


        pub fun setMysteryBoxTypeToList(typeId: UInt64,
                                        describe: String,
                                        stock: UInt64,
                                        unitPrice: UFix64){
            pre {
                typeId > 0 : "MysteryBox type should > 0"
                describe != "" : "MysteryBox describe can not be empty"
                stock >= 0 : "MysteryBox stock should >= 0"
                unitPrice > 0.0 : "MysteryBox type should > 0.0"
            }
            MysteryBox.mysteryBoxTypeList[typeId] = MysteryBoxType(typeId: typeId,
                                                                   describe: describe,
                                                                   stock: stock,
                                                                   unitPrice: unitPrice)
        }

        pub fun addTotalSupply(amount: UInt64){
            pre {
                amount >= 0 : "amount should > 0"
            }
            MysteryBox.totalSupply = MysteryBox.totalSupply + amount
        }

        pub fun setPrizePool(tokenName: String, amount: UFix64){
            pre {
                tokenName != "" : "MysteryBox type should > 0"
                amount >= 0.0 : "amount should >= 0.0"
            }
            MysteryBox.prizePool[tokenName] = amount
            emit UpdatePrizePool(tokenName: tokenName, amount: amount)
        }

        pub fun setMainFusdReceiver(mainFusdReceiverAddr: Address) {
            self.mainFusdReceiverAddr = mainFusdReceiverAddr
        }

        pub fun setMysteryBoxNFTPrividerCap(prividerCap: Capability<&NyatheesOVO.NFTMinter>){
            
            self.nftProviderCap = prividerCap
        }

//public methods
        pub fun getTotalSupply():UInt64{
            return MysteryBox.totalSupply
        }

        pub fun getAccountItem(address: Address): accountItem?{
            return MysteryBox.accountList[address]
        }

        pub fun getPrizePoolBalance(tokenName: String): UFix64{
            return MysteryBox.prizePool[tokenName]!
        }

        pub fun getAccountInfo(address: Address): accountItem?{
            return MysteryBox.accountList[address]
        }

        pub fun getMysteryBoxTypeItem(typeId: UInt64): MysteryBox.MysteryBoxType?{
            return MysteryBox.mysteryBoxTypeList[typeId]
        }

        init(){
            self.mainFusdReceiverAddr = MysteryBox.ZERO_ADDRESS
            self.nftProviderCap = nil
        }

}

    init(){
        self.prizePool = {}
        self.accountList = {}
        self.mysteryBoxTypeList = {}
        self.totalSupply = 0 as UInt64
        self.MysteryBoxControllerPublicPath = /public/MysteryBoxControllerPublic
        self.MysteryBoxControllerPrivatePath = /private/MysteryBoxControllerPrivate
        self.MysteryBoxControllerStoragePath = /storage/MysteryBoxController
        self.ZERO_ADDRESS = 0x0000000000000000
        self.maxMysteryBoxToBuy = 10

        let mysteryBoxController <- create MysteryBoxController()
        self.account.save(<-mysteryBoxController, to: self.MysteryBoxControllerStoragePath)
        self.account.link<&MysteryBox.MysteryBoxController{MysteryBoxControllerPrivate}>(self.MysteryBoxControllerPrivatePath, target: self.MysteryBoxControllerStoragePath)
        self.account.link<&MysteryBox.MysteryBoxController{MysteryBoxControllerPublic}>(self.MysteryBoxControllerPublicPath, target: self.MysteryBoxControllerStoragePath)
    }

}
