//这是根据官方NFT接口实现的NftEggs项目的NFT合约

import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

// import NonFungibleToken from 0xf8d6e0586b0a20c7
// import FungibleToken from 0xee82856bf20e2aa6

pub contract NftEggsNFT: NonFungibleToken {
	
    pub let MinterStoragePath: StoragePath
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath	
	// 此类型的token总数
    pub var totalSupply: UInt64
	// 初始化NFT合约时发出的事件
    pub event ContractInitialized()
    // 创建新item结构时触发
    pub event ItemCreated(id: UInt64, metadata: {String:String})	
	// 当NFT铸造时触发
    pub event NftEggsNFTMinted(id: UInt64, itemId: UInt64, serialNumber: UInt64)
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // item结构的数据字典
    access(self) var itemDatas: {UInt64: Item}
	
	// 创建item时使用的id.
    // 每新建一个item，nextItemId加1
    pub var nextItemId: UInt64
	
    // Item是一个结构，它保存与NFT关联的元数据
    pub struct Item {

        // 唯一id
        pub let itemId: UInt64
        pub let quantity: UInt64
        pub let royalty: UFix64
        pub let creator: Address

        // 将有关该项的所有元数据存储为字符串映射。
        pub let metadata: {String: String}

        init(metadata: {String: String}, quantity: UInt64, royalty: UFix64, creator: Address) {
            pre {
                metadata.length > 0: "metadata cannot be empty"
                quantity>0 : "quantity must greater than 0"
                royalty>=0.0 : "royalty must greater than or equal to 0"
                royalty<=0.3 : "royalty must less than or equal to 30"
            }
            self.itemId = NftEggsNFT.nextItemId
            self.metadata = metadata
            self.quantity = quantity
            self.royalty = royalty
            self.creator = creator

            // 每新建一个item，nextItemId加1，确保itemId唯一
            NftEggsNFT.nextItemId = NftEggsNFT.nextItemId + (1 as UInt64)

            emit ItemCreated(id: self.itemId, metadata: metadata)
        }
    }
	// NFT交易费用
    pub struct Royalty{
        pub let wallet:Capability<&{FungibleToken.Receiver}> 
        pub let cut: UFix64

        init(wallet:Capability<&{FungibleToken.Receiver}>, cut: UFix64 ){
           self.wallet=wallet
           self.cut=cut
        }
    }
	
	// NFT元数据结构体
    pub struct NftEggsNFTData {


        // 铸造NFT的item的id
        pub let itemId: UInt64

        // NFT铸造序号，同一个item铸造多个NFT，每个NFT不同的序号，其他元数据相同
        pub let serialNumber: UInt64

        init(itemId: UInt64, serialNumber: UInt64) {
            self.itemId = itemId
            self.serialNumber = serialNumber
        }
    }

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        // NFT元数据结构体
        pub let metadata: NftEggsNFTData
        pub let royalty: {String: Royalty}

        init(itemId: UInt64, serialNumber: UInt64, royalty: {String: Royalty}) {
            NftEggsNFT.totalSupply = NftEggsNFT.totalSupply + (1 as UInt64);
			self.id = NftEggsNFT.totalSupply
            self.metadata = NftEggsNFTData(itemId: itemId, serialNumber: serialNumber)
            self.royalty = royalty
			emit NftEggsNFTMinted(id: self.id, itemId: itemId, serialNumber: serialNumber)
        }

    }
	
    // 公开接口，以便其他用户可以查询NFT信息
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
		pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowNftEggsNFT(id: UInt64): &NftEggsNFT.NFT? {
			//如果结果不是nil，则返回的引用的id应与函数的参数相同
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow NftEggsNFT reference: The id of the returned reference is incorrect."
            }
        }
    }
	
    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // 用于存储NFT信息集合的词典
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
		// 初始化时，将NFT信息集合初始化为空
        init () {
            self.ownedNFTs <- {}
        }

        // 提取NFT并转移给调用者
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // 存储NFT
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @NftEggsNFT.NFT

            let id: UInt64 = token.id

            // 将新的token存储到集合（如果id相同会被替换）
            let oldToken <- self.ownedNFTs[id] <- token
			if self.owner?.address != nil {
				emit Deposit(id: id, to: self.owner?.address)
			}
            destroy oldToken
        }

        // 一次性存入多个NFT
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {

            let keys = tokens.getIDs()

            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }
            destroy tokens
        }

        // getIDs返回集合中的ID数组
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

		//返回对集合中NFT的引用
		//以便调用者可以读取数据并从中调用方法
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // 返回对NFT的引用，调用者可以读取 metadata,
        pub fun borrowNftEggsNFT(id: UInt64): &NftEggsNFT.NFT? {
			if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &NftEggsNFT.NFT
            } else {
                return nil
            }
        }				
		
		//销毁NFT集合
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // 创建一个空集合，并将其返回给调用者，以便他们可以拥有NFT
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create NftEggsNFT.Collection()
    }

    // 以管理员身份铸造和分发代币
    pub resource NFTMinter {

        // 使用新ID创建一个新的NFT，并使用收件人的集合引用将其存放在收件人集合中
        priv fun _mintNFT(itemId: UInt64, serial: UInt64, royalty:{String: Royalty}) : @NFT  {
			let nft : @NFT  <- create NftEggsNFT.NFT( itemId:itemId, serial: serial, royalty:royalty)
			return <-nft
        }
		
        // 使用相同的元数据创建指定个数的NFT，并使用收件人的集合引用将其存放在收件人集合中
        pub fun mintNFT(recipient: &{NftEggsNFT.CollectionPublic}, metadata: {String: String}, quantity: UInt64, rate: UFix64) {

			let creator = recipient.owner!.address
            var newItem = Item(metadata: metadata, quantity:quantity, royalty:rate, creator: creator)
            let newId = newItem.itemId
            NftEggsNFT.itemDatas[newId] = newItem			
			
			let creatorWallet= getAccount(creator).getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
            let platformWallet=  NftEggsNFT.account.getCapability<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)

            let royalty = {
                "creator" : NftEggsNFT.Royalty(wallet: creatorWallet, cut: rate),
                "platform" : NftEggsNFT.Royalty(wallet: platformWallet, cut: 0.1)
            }
			
			let newCollection <- create Collection()
			var i: UInt64 = 0
            while i < quantity {
                i = i + (1 as UInt64)
				newCollection.deposit(token: <-self._mintNFT(itemId:newId, serial:i, royalty:royalty))
            }
			
			recipient.batchDeposit(tokens: <-newCollection)
        }		
		
    }
	
    // 返回所有Items
    pub fun getAllItems(): [NftEggsNFT.Item] {
        return NftEggsNFT.itemDatas.values
    }

    // 返回指定Item的元数据
    pub fun getItemMetadata(itemId: UInt64): {String: String}? {
        return self.itemDatas[itemId]?.metadata
    }
	
    // 返回指定Item的版权费率
    pub fun getItemRoyalty(itemId: UInt64): UFix64? {
        return self.itemDatas[itemId]?.royalty
    } 
	
	// 返回指定Item的创建人
    pub fun getItemCreator(itemId: UInt64): Address? {
        return self.itemDatas[itemId]?.creator
    }
		
	// 返回指定Item铸造NFT总数
    pub fun getItemQuantity(itemId: UInt64): UInt64? {
        return self.itemDatas[itemId]?.quantity
    }
	
    // 返回指定Item的元数据的指定字段
    pub fun getItemMetadataByField(itemId: UInt64, field: String): String? {
        if let item = NftEggsNFT.itemDatas[itemId] {
            return item.metadata[field]
        } else {
            return nil
        }
    }	

    init() {
        // 初始化此类型NFT数量
        self.totalSupply = 0
        self.CollectionPublicPath = /public/NftEggsNFTCollectionV2
        self.CollectionPrivatePath = /private/NftEggsNFTCollectionV2
        self.CollectionStoragePath = /storage/NftEggsNFTCollectionV2
        self.MinterStoragePath = /storage/NftEggsNFTMinterV2
		self.itemDatas = {}
        self.nextItemId = 1
        self.account.save<@Collection>(<- create NftEggsNFT.Collection(), to: NftEggsNFT.CollectionStoragePath)
        self.account.link<&{CollectionPublic}>(NftEggsNFT.CollectionPublicPath, target: NftEggsNFT.CollectionStoragePath)
		
        self.account.save(<- create NftEggsNFT.NFTMinter(), to: NftEggsNFT.MinterStoragePath)

        emit ContractInitialized()
    }
}

