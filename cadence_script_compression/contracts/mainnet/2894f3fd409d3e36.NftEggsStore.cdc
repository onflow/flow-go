import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import NftEggsNFT from 0x2894f3fd409d3e36

//import NftEggsNFT from 0xf8d6e0586b0a20c7
//import NonFungibleToken from 0xf8d6e0586b0a20c7
//import FlowToken from 0x0ae53cb6e3f42a79
//import FungibleToken from 0xee82856bf20e2aa6

pub contract NftEggsStore {

    pub event NftEggsStoreInitialized()

    pub event StorefrontInitialized(storefrontResourceID: UInt64)

    pub event StorefrontDestroyed(storefrontResourceID: UInt64)

    // 列表是否可用
    // 已创建列表并将其添加到店面资源中。
	// 此处的地址值在发出事件时有效，但他们所指的账户状态在可能会在NftEggsStore工作流外部发生变化，因此在使用它们时请仔细检查。
    //
    pub event ListingAvailable(
        storefrontAddress: Address,
        listingResourceID: UInt64,
        nftType: Type,
        nftID: UInt64,
        ftVaultType: Type,
        price: UFix64,
        saleType: UInt64,
        minimumBidIncrement: UFix64,
        startTime: UFix64,
        endTime: UFix64
    )

	//列表已完成
	//它要么被购买，要么被移除并销毁。
    //
    pub event ListingCompleted(
        storefrontAddress: Address?,
        listingResourceID: UInt64,
        storefrontResourceID: UInt64,
        purchased: Bool,
        buyerAddress: Address?,
        dealPrice: UFix64?,
        nftID: UInt64
    )
	pub event Bid(
        storefrontAddress: Address,
        listingResourceID: UInt64,
        storefrontResourceID: UInt64,
        bidAddress: Address,
        bidPrice: UFix64,
        nftID: UInt64,
        endTime: UFix64
    )
	
    pub let StorefrontStoragePath: StoragePath
    pub let StorefrontPublicPath: PublicPath


	//列表详细信息
	//包含列表数据的结构。
    //
    pub struct ListingDetails {
        // 存储列表的店面id
        pub var storefrontID: UInt64
        // 此列表是否已购买
        pub var purchased: Bool
        // 列表中NFT的类型
        pub let nftType: Type
        // 该类型中NFT的ID
        pub let nftID: UInt64
        // 付款的代币的类型。
        pub let salePaymentVaultType: Type
        // 初始销售价格，拍卖成交价以最终bidPrice为准
        pub let salePrice: UFix64
        // 支付的代币在多个收款人中怎么分配（创建人收取版权费，交易平台收取佣金，剩下部分给原持有人）
        pub let saleCuts: {String: NftEggsNFT.Royalty}
        // 销售类型1一口价，2拍卖
        pub let saleType: UInt64
        //开始时间
        pub var startTime: UFix64
        //结束时间
        pub var endTime: UFix64
		//拍卖投标次数
        pub var numberOfBids: UInt64
        //每次投标的最小增量
        pub let minimumBidIncrement: UFix64		
        //下次投标的值
        pub var minNextBid: UFix64		
        //当前投标的金额
        pub var bidPrice: UFix64	
        //当前投标的地址
        pub var bidAddress: Address?


        // 不可逆转地将此列表设置为已购买
        access(contract) fun setToPurchased() {
            self.purchased = true
        }

        access(contract) fun setBid( endTime: UFix64, bidPrice: UFix64, bidAddress: Address ) {
            self.numberOfBids=self.numberOfBids+(1 as UInt64)
			self.endTime = endTime
            self.bidPrice = bidPrice
            self.bidAddress = bidAddress
            self.minNextBid = bidPrice + self.minimumBidIncrement
        }

        init (
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
			salePrice: UFix64,
            saleCuts: {String: NftEggsNFT.Royalty},
            saleType: UInt64,
			startTime:UFix64,
			endTime:UFix64,
			minimumBidIncrement:UFix64,
			storefrontID: UInt64
        ) {
            self.storefrontID = storefrontID
            self.purchased = false
            self.nftType = nftType
            self.nftID = nftID
            self.salePaymentVaultType = salePaymentVaultType
            self.saleCuts = saleCuts
            self.salePrice = salePrice
            self.saleType = saleType
            self.startTime = startTime
            self.endTime = endTime
            self.minimumBidIncrement = minimumBidIncrement
            self.numberOfBids = 0
            self.bidAddress = nil
            self.bidPrice = 0.0
            self.minNextBid = salePrice + minimumBidIncrement
        }
		
        //是否过了结束时间
        pub fun isExpired() : Bool {
            let endTime = self.endTime
            let currentTime = getCurrentBlock().timestamp
            let remaining= Fix64(currentTime) - Fix64(endTime)
            return remaining>0.0
        }		
    }


    // 为列表提供有用的公共接口的接口。
    //
    pub resource interface ListingPublic {
        // 对NFT的引用
        pub fun borrowNFT(): &NonFungibleToken.NFT

		//购买
        pub fun purchase(payment: @FungibleToken.Vault, buyerAddress: Address)
		//投标
		pub fun placeBid(bidTokens: @FungibleToken.Vault, address: Address)
        // getDetails
        pub fun getDetails(): ListingDetails
    }


	//上市
    pub resource Listing: ListingPublic {
        
		//当前投标用户钱包引用
		access(self) var bidVault: @FungibleToken.Vault
		
        access(self) let details: ListingDetails

		//允许此资源从其集合中提取具有给定ID的NFT的功能。
		//此功能允许资源提取*任何*NFT，因此在提供时应小心
		//这样一种能力可以应用于资源，并始终检查其代码，以确保它将在未来以它声称的方式使用它
        access(contract) let nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>

        // 对NFT的引用
        pub fun borrowNFT(): &NonFungibleToken.NFT {
            let ref = self.nftProviderCapability.borrow()!.borrowNFT(id: self.getDetails().nftID)

            assert(ref.isInstance(self.getDetails().nftType), message: "token has wrong type")
            assert(ref.id == self.getDetails().nftID, message: "token has wrong ID")
            return ref as &NonFungibleToken.NFT
        }

		//获取详细信息
		//以结构形式获取列表当前状态的详细信息。
		//这避免了为它们提供更多的公共变量和getter方法，并很好地使用脚本（不能返回资源）发挥了作用
        pub fun getDetails(): ListingDetails {
            return self.details
        }

		//购买
        pub fun purchase(payment: @FungibleToken.Vault, buyerAddress: Address) {
            
			pre {
                self.details.purchased == false: "listing has already been purchased"
                self.details.saleType == 1: "listing's saleType is bid"
                payment.isInstance(self.details.salePaymentVaultType): "payment vault is not requested fungible token"
                payment.balance == self.details.salePrice: "payment vault does not contain requested price"
            }
			
			let now = getCurrentBlock().timestamp
			assert(self.details.startTime<now, message: "NFT does not start to sale")
			assert(self.details.endTime>now, message: "NFT is end to sale")
            // 获取NFT以转给购买者
            let nft <-self.nftProviderCapability.borrow()!.withdraw(withdrawID: self.details.nftID)
			//接收者和提供者都不可信，他们必须实现正确的接口，但除遵守其前置/后置条件外，它们可以不受限制的以任何给定的方式实现接口背后的功能。
			//因此我们不能信任接口后面的收集资源，我们必须检查它提供给我们的NFT资源，以确保它是正确的。
            assert(nft.isInstance(self.details.nftType), message: "withdrawn NFT is not of specified type")
            assert(nft.id == self.details.nftID, message: "withdrawn NFT does not have specified ID")
            assert(self.owner?.address! != buyerAddress, message: "can't buy your own")

		    // 设置被购买
            self.details.setToPurchased()

			//如果我们试图付款时没有任何接收人有效，我们将发送给第一个有效的接收者而不是中止交易，
			//因此，第一个接收人应当是卖方，或者是合同的约定接收人
            var residualReceiver: &{FungibleToken.Receiver}? = nil

            // 向每个收款人支付他们声明的付款金额
			if let ownerReceiver = self.details.saleCuts["owner"]!.wallet.borrow() {
				let paymentCut <- payment.withdraw(amount: self.details.saleCuts["owner"]!.cut * self.details.salePrice)
				ownerReceiver.deposit(from: <-paymentCut)
				if (residualReceiver == nil) {
					residualReceiver = ownerReceiver
				}
			}
            
			if let platformReceiver = self.details.saleCuts["platform"]!.wallet.borrow() {
				let paymentCut <- payment.withdraw(amount: self.details.saleCuts["platform"]!.cut * self.details.salePrice)
				platformReceiver.deposit(from: <-paymentCut)
				if (residualReceiver == nil) {
					residualReceiver = platformReceiver
				}
			}
			
			if(self.details.saleCuts["creator"]!.cut>0.0){           
				if let creatorReceiver = self.details.saleCuts["creator"]!.wallet.borrow() {
					let paymentCut <- payment.withdraw(amount: self.details.saleCuts["creator"]!.cut * self.details.salePrice)
					creatorReceiver.deposit(from: <-paymentCut)
					if (residualReceiver == nil) {
						residualReceiver = creatorReceiver
					}
				}
			}
			
            assert(residualReceiver != nil, message: "No valid payment receivers")

			//此时，如果所有接收者都处于活动状态且可用，则支付保险库将具有
			//剩下的令牌为零，从功能上讲，这将是一个消耗空保险库的无操作
            residualReceiver!.deposit(from: <-payment)
            // 对收件人的NFT集合的引用
            let depositRef = getAccount(buyerAddress).getCapability(NftEggsNFT.CollectionPublicPath)
                .borrow<&{NftEggsNFT.CollectionPublic}>()
                ?? panic("Could not borrow a reference to the receiver's collection")
            // 存入收件人NFT集合
            depositRef.deposit(token: <-nft)

			//如果购买了该清单，我们将其视为在此完成。
			//否则我们认为它在析构函数中完成。
            emit ListingCompleted(
                storefrontAddress: self.owner?.address!,
                listingResourceID: self.uuid,
                storefrontResourceID: self.details.storefrontID,
                purchased: self.details.purchased,
                buyerAddress: buyerAddress,
                dealPrice: self.details.salePrice,
                nftID: self.details.nftID
            )

        }
		
        pub fun placeBid(bidTokens: @FungibleToken.Vault,address: Address) {
            

			pre {
                self.details.purchased == false: "listing has already been purchased"
                self.details.saleType == 2: "listing's saleType is not bid"
                bidTokens.isInstance(self.details.salePaymentVaultType): "payment vault is not requested fungible token"
            }
			
            assert(self.owner?.address! != address, message: "can't bid your own")

            if bidTokens.balance < self.details.minNextBid {
                panic("bid amount must be larger or equal to the current price + minimum bid increment ".concat(bidTokens.balance.toString()).concat(" < ").concat(self.details.minNextBid.toString()))
            }
            let now = getCurrentBlock().timestamp
			assert(self.details.startTime<now, message: "NFT does not start to bid")
			assert(self.details.endTime>now, message: "NFT is end to bid")


			//将前一个投标人的代币退回
			if self.bidVault.balance > 0.0 {
				let returnVault = getAccount(self.details.bidAddress!).getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
                returnVault.borrow()!.deposit(from: <-self.bidVault.withdraw(amount: self.bidVault.balance))
			}
            
			let oldVault <- self.bidVault <- bidTokens
            destroy oldVault

            // 替换为当前投标人的信息
			var endTime = self.details.endTime
			if endTime < now+300.0 {
				endTime = now + 300.0
			}

			self.details.setBid(
				endTime: endTime,
                bidPrice: self.bidVault.balance,
                bidAddress: address
			)

            emit Bid(
                storefrontAddress: self.owner?.address!,
                listingResourceID: self.uuid,
                storefrontResourceID: self.details.storefrontID,
                bidAddress: self.details.bidAddress!,
                bidPrice: self.details.bidPrice,
                nftID: self.details.nftID,
                endTime: self.details.endTime
            )

        }	

		
        //结束销售
        pub fun endSale(owner: Bool)  {

            pre {
                !self.details.purchased : "The NFT has saled"
                
            }

            if (!owner) && (!self.details.isExpired()) {
                panic("The Listing has not completed yet")
            }

            //拍卖且有人出价的处理
            if self.details.saleType != 1 && self.details.bidPrice > 0.0 {
                
                //self.pay(payment: <-self.bidVault)
                //let payment <- self.bidVault
                // 设置被购买
                self.details.setToPurchased()

                //如果我们试图付款时没有任何接收人有效，我们将发送给第一个有效的接收者而不是中止交易，
                //因此，第一个接收人应当是卖方，或者是合同的约定接收人
                var residualReceiver: &{FungibleToken.Receiver}? = nil

                // 向每个收款人支付他们声明的付款金额
                if let ownerReceiver = self.details.saleCuts["owner"]!.wallet.borrow() {
                    let paymentCut <- self.bidVault.withdraw(amount: self.details.saleCuts["owner"]!.cut * self.details.bidPrice)
                    ownerReceiver.deposit(from: <-paymentCut)
                    if (residualReceiver == nil) {
                        residualReceiver = ownerReceiver
                    }
                }
                
                if let platformReceiver = self.details.saleCuts["platform"]!.wallet.borrow() {
                    let paymentCut <- self.bidVault.withdraw(amount: self.details.saleCuts["platform"]!.cut * self.details.bidPrice)
                    platformReceiver.deposit(from: <-paymentCut)
                    if (residualReceiver == nil) {
                        residualReceiver = platformReceiver
                    }
                }
                
                if(self.details.saleCuts["creator"]!.cut>0.0){           
                    if let creatorReceiver = self.details.saleCuts["creator"]!.wallet.borrow() {
                        let paymentCut <- self.bidVault.withdraw(amount: self.details.saleCuts["creator"]!.cut * self.details.bidPrice)
                        creatorReceiver.deposit(from: <-paymentCut)
                        if (residualReceiver == nil) {
                            residualReceiver = creatorReceiver
                        }
                    }
                }
                
                assert(residualReceiver != nil, message: "No valid payment receivers")

                //此时，如果所有接收者都处于活动状态且可用，则支付保险库将具有
                //剩下的令牌为零，从功能上讲，这将是一个消耗空保险库的无操作
                //residualReceiver!.deposit(from: <-self.bidVault)
    

                // 获取NFT以转给购买者
                let nft <-self.nftProviderCapability.borrow()!.withdraw(withdrawID: self.details.nftID)
                //接收者和提供者都不可信，他们必须实现正确的接口，但除遵守其前置/后置条件外，它们可以不受限制的以任何给定的方式实现接口背后的功能。
                //因此我们不能信任接口后面的收集资源，我们必须检查它提供给我们的NFT资源，以确保它是正确的。
                assert(nft.isInstance(self.details.nftType), message: "withdrawn NFT is not of specified type")
                assert(nft.id == self.details.nftID, message: "withdrawn NFT does not have specified ID")
                // 对收件人的NFT集合的引用
                let depositRef = getAccount(self.details.bidAddress!).getCapability(NftEggsNFT.CollectionPublicPath)
                    .borrow<&{NftEggsNFT.CollectionPublic}>()
                    ?? panic("Could not borrow a reference to the receiver's collection")
                // 存入收件人NFT集合
                depositRef.deposit(token: <-nft)

                emit ListingCompleted(
                    storefrontAddress: self.owner?.address,
                    listingResourceID: self.uuid,
                    storefrontResourceID: self.details.storefrontID,
                    purchased: self.details.purchased,
                    buyerAddress: self.details.bidAddress!,
                    dealPrice: self.details.bidPrice,
                    nftID: self.details.nftID
                )
			
			}
        }


        destroy () {
            if !self.details.purchased {
                emit ListingCompleted(
                    storefrontAddress: self.owner?.address,
                    listingResourceID: self.uuid,
                    storefrontResourceID: self.details.storefrontID,
                    purchased: self.details.purchased,
                    buyerAddress: nil,
                    dealPrice: nil,
                    nftID: self.details.nftID
                )
            }
            destroy self.bidVault
        }

        // initializer
        //
        init (
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleItemPrice: UFix64,
            saleCuts: {String: NftEggsNFT.Royalty},
			saleType: UInt64,
			startTime: UFix64,
			endTime: UFix64,
			minimumBidIncrement: UFix64,
            storefrontID: UInt64
        ) {

            if saleType == 2 && (! (minimumBidIncrement>0.0)) {
                panic("when saleType is 2, minimumBidIncrement must greater then 0")
            }

            // 存储销售信息
            self.details = ListingDetails(
                nftType: nftType,
                nftID: nftID,
                salePaymentVaultType: salePaymentVaultType,
                salePrice: saleItemPrice,
                saleCuts: saleCuts,
				saleType: saleType,
				startTime: startTime,
				endTime: endTime,
				minimumBidIncrement: minimumBidIncrement,	
                storefrontID: storefrontID
            )

            // 存储对NFT操作能力的引用
            self.nftProviderCapability = nftProviderCapability
			self.bidVault <- FlowToken.createEmptyVault()

			//检查提供程序是否包含NFT。
			//代币售出后，我们会再次检查。
			//我们不能将其移动到函数中，因为初始值设定项不能调用成员函数。
            let provider = self.nftProviderCapability.borrow()
            assert(provider != nil, message: "cannot borrow nftProviderCapability")

            // 检查NFT是否有效。
            let nft = provider!.borrowNFT(id: self.details.nftID)
            assert(nft.isInstance(self.details.nftType), message: "token is not of specified type")
            assert(nft.id == self.details.nftID, message: "token does not have specified ID")
        }
    }

	//店面管理资源
	//用于在店面中添加和删除列表
    pub resource interface StorefrontManager {
        // 创建销售列表项
        // 允许店面拥有者创建并插入信息到销售列表
        pub fun createListing(
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleItemPrice: UFix64,
			saleType: UInt64,
			startTime: UFix64,
			endTime: UFix64,
			minimumBidIncrement: UFix64	
        ): UInt64
        // removeListing
        // 允许店面所有者删除任何销售清单
        pub fun removeListing(listingResourceID: UInt64)
    }



	//店面公众功能接口
    pub resource interface StorefrontPublic {
        pub fun getListingIDs(): [UInt64]
        pub fun borrowListing(listingResourceID: UInt64): &Listing{ListingPublic}?
        pub fun cleanup(listingResourceID: UInt64)
   }

	//店面
	//一种资源，允许其所有者管理列表，并允许任何人与列表交互
	//以便查询他们的详细信息并购买他们所代表的NFT。
    pub resource Storefront : StorefrontManager, StorefrontPublic {
        // 列出UUID的字典用于列出资源
        access(self) var listings: @{UInt64: Listing}

        // 创建销售列表项
        // 允许店面拥有者创建并插入信息到销售列表
        pub fun createListing(
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
			saleItemPrice: UFix64,
            saleType: UInt64,
			startTime: UFix64,
			endTime: UFix64,
			minimumBidIncrement: UFix64			
         ): UInt64 {
			if endTime<=startTime {
                panic("startTime must be less than endTime")
            }
			let now = getCurrentBlock().timestamp
            if endTime<now {
                panic("endTime must be  greater than now")
            }
			let collectionRef = getAccount(nftProviderCapability.address)
					.getCapability(NftEggsNFT.CollectionPublicPath)
					.borrow<&{NftEggsNFT.CollectionPublic}>()
					?? panic("Could not borrow capability from public collection")

			let nft = collectionRef.borrowNftEggsNFT(id: nftID)
			
			let ownerWallet = getAccount(nftProviderCapability.address).getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
			let royalty = {
				"owner" : NftEggsNFT.Royalty(wallet: ownerWallet, cut: (1.0 - nft!.royalty["creator"]!.cut - nft!.royalty["platform"]!.cut)),
				"platform" : nft!.royalty["platform"]!,
				"creator" : nft!.royalty["creator"]!
			}
				
			let listing <- create Listing(
                nftProviderCapability: nftProviderCapability,
                nftType: nftType,
                nftID: nftID,
                salePaymentVaultType: salePaymentVaultType,
				saleItemPrice: saleItemPrice,
                saleCuts: royalty,
				saleType: saleType,
				startTime: startTime,
				endTime: endTime,
				minimumBidIncrement: minimumBidIncrement,	
                storefrontID: self.uuid
            )

            let listingResourceID = listing.uuid
            let listingPrice = listing.getDetails().salePrice

            // Add the new listing to the dictionary.
            let oldListing <- self.listings[listingResourceID] <- listing
            // Note that oldListing will always be nil, but we have to handle it.
            destroy oldListing

            emit ListingAvailable(
                storefrontAddress: self.owner?.address!,
                listingResourceID: listingResourceID,
                nftType: nftType,
                nftID: nftID,
                ftVaultType: salePaymentVaultType,
                price: listingPrice,
				saleType: saleType,
				minimumBidIncrement: minimumBidIncrement,
				startTime: startTime,
				endTime: endTime
            )

            return listingResourceID
        }

        // removeListing
        // Remove a Listing that has not yet been purchased from the collection and destroy it.
        //
        pub fun removeListing(listingResourceID: UInt64) {
            pre {
                self.listings[listingResourceID] != nil: "could not find listing with given id"
            }

            let listing <- self.listings.remove(key: listingResourceID)!
				
			if listing.getDetails().saleType != 1 {
				listing.endSale(owner: true)
			}
			
            // This will emit a ListingCompleted event.
            destroy listing
        }

        // getListingIDs
        // Returns an array of the Listing resource IDs that are in the collection
        //
        pub fun getListingIDs(): [UInt64] {
            return self.listings.keys
        }

        // borrowSaleItem
        // Returns a read-only view of the SaleItem for the given listingID if it is contained by this collection.
        //
        pub fun borrowListing(listingResourceID: UInt64): &Listing{ListingPublic}? {
            if self.listings[listingResourceID] != nil {
                return &self.listings[listingResourceID] as! &Listing{ListingPublic}
            } else {
                return nil
            }
        }

        // cleanup
        // 如果已经被购买，可以移除.
        // Anyone can call, but at present it only benefits the account owner to do so.
        // Kind purchasers can however call it if they like.
        //
        pub fun cleanup(listingResourceID: UInt64) {
            pre {
                self.listings[listingResourceID] != nil: "could not find listing with given id"
            }
            let listing <- self.listings.remove(key: listingResourceID)!
            let now = getCurrentBlock().timestamp

            if listing.getDetails().endTime>now {
                assert(listing.getDetails().purchased == true, message: "listing is not purchased, only admin can remove")
            }
			if listing.getDetails().saleType != 1 {
				listing.endSale(owner: false)
			}
            destroy listing
        }

        // destructor
        //
        destroy () {
            destroy self.listings

            // Let event consumers know that this storefront will no longer exist
            emit StorefrontDestroyed(storefrontResourceID: self.uuid)
        }

        // constructor
        //
        init () {
            self.listings <- {}

            // Let event consumers know that this storefront exists
            emit StorefrontInitialized(storefrontResourceID: self.uuid)
        }
    }

    // createStorefront
    // Make creating a Storefront publicly accessible.
    //
    pub fun createStorefront(): @Storefront {
        return <-create Storefront()
    }

    init () {
        self.StorefrontStoragePath = /storage/NftEggsStoreV2
        self.StorefrontPublicPath = /public/NftEggsStoreV2

        emit NftEggsStoreInitialized()
    }
}
