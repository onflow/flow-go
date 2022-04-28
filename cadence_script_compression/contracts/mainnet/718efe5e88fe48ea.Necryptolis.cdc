import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract Necryptolis: NonFungibleToken {

    pub event ContractInitialized()
    
    pub event Withdraw(id: UInt64, from: Address?)
    
    // Emitted when a necryptolis nft is deposited in a collection
    pub event Deposit(id: UInt64, to: Address?)
    
    // Emitted when a necryptolis nft is deposited in a collection with slightly more data
    pub event DepositNecryptolisNFT(id: UInt64, to: Address?, left: Int32, top: Int32)
    pub event CemeteryPlotMinted(id: UInt64, plotData: PlotData)
    pub event Minted(id: UInt64, typeID: UInt64)

    // Emitted whenever plotSalesInfo has changed (Price changes, min/max plot dimensions changes)
    pub event PlotSalesInfoChanged(
        squarePixelPrice: UFix64, 
        candlePrice: UFix64, 
        trimPrice: UFix64, 
        maxPlotHeight: UInt16, 
        maxPlotWidth: UInt16, 
        minPlotHeight: UInt16, 
        minPlotWidth: UInt16, 
        vaultAddress: Address,
        vaultType: Type)

    // Emitted when a Plot receives it's Gravestone
    pub event GravestoneCreated(id: UInt64, name: String, fromDate: String, toDate: String, metadata: {String:String}, left: Int32, top: Int32)
    // Emitted when ToDate has been added to an already existing date
    pub event ToDateSet(id: UInt64, toDate: String, left: Int32, top: Int32)
    // Emitted when someone buys a candle for an NFT
    pub event CandleLit(id: UInt64, left: Int32, top: Int32, buyerAddress: Address)
    // Emitted whenever plot is trimmed
    pub event CemeteryPlotTrimmed(id: UInt64, left: Int32, top: Int32)
    // Emitted when someone buries another NFT inside Necryptolis NFT
    pub event NFTBuried(id: UInt64, left: Int32, top: Int32, nftID: UInt64, nftType: Type)
    
    pub var totalSupply: UInt64

    // Url used for metadata of NFT thumbnail property
    pub var imagesBaseURL: String

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let ResolverCollectionPublicPath : PublicPath
    pub let NecryptolisAdminStoragePath: StoragePath
    pub let PlotMinterStoragePath: StoragePath
    pub let GravestoneManagerStoragePath: StoragePath

    // information about all the plots in Necryptolis (their position and dimension)
    // necryptolis is divided into 1000x1000 sections
    // {xSection : {ySection : { nftId: PlotData }}}
    pub let plotDatas: {Int32: {Int32 : {UInt64: PlotData}}}     

    pub let plotSalesInfo : PlotSalesInfo
    
    // Holds all the restrictions and prices for creating Necryptolis NFTs
    pub struct PlotSalesInfo {
            pub(set) var squarePixelPrice: UFix64
            pub(set) var candlePrice: UFix64
            pub(set) var trimPrice: UFix64
            pub(set) var maxPlotHeight: UInt16
            pub(set) var maxPlotWidth: UInt16
            pub(set) var minPlotHeight: UInt16
            pub(set) var minPlotWidth: UInt16

            // When someone buys a service (candles/trimming), this vault gets the tokens
            pub(set) var servicesProviderVault: Capability<&AnyResource{FungibleToken.Receiver}>?

            init(squarePixelPrice: UFix64, candlePrice: UFix64, trimPrice: UFix64, maxPlotHeight: UInt16, maxPlotWidth: UInt16, minPlotHeight: UInt16, minPlotWidth: UInt16, vault: Capability<&AnyResource{FungibleToken.Receiver}>?){
                self.squarePixelPrice = squarePixelPrice
                self.candlePrice = candlePrice  
                self.trimPrice = trimPrice
                self.maxPlotHeight = maxPlotHeight
                self.maxPlotWidth = maxPlotWidth
                self.minPlotHeight = minPlotHeight
                self.minPlotWidth = minPlotWidth                
                self.servicesProviderVault = vault
            }
    }

    // We split Necryptolis surface into square blocks of 1000 pixels
    // this helper function returns a section for a given x or y coordinate
    pub fun getSection(position: Int32) : Int32 {
        return position / 1000;
    }

    // further away the cemetery plot is created the price drops
    // this helper method returns the factor by which the price needs to be reduced
    pub fun getPlotDistanceFactor(left: Int32, top: Int32) : UFix64 {
        var valueA = left
        var valueB = top
        if(left < 0){
            valueA = left * -1
        }
        if(top < 0){
            valueB = top * -1
        }

        var biggerValue : Int32 = valueA
        if valueB > valueA {
            biggerValue = valueB
        }
                
        return 1.0 / UFix64(biggerValue / 1000 + 1)
    }

    access(contract) fun isPlotCollidingInSection(xSection: Int32, ySection: Int32, left: Int32, top: Int32, width: UInt16, height: UInt16) : Bool {
        if(Necryptolis.plotDatas[xSection] == nil){        
            return false
        }
        let plotDatasYSection = Necryptolis.plotDatas[xSection]!
        
        if(plotDatasYSection[ySection] == nil){
            return false
        }

        let plotsInSection = plotDatasYSection[ySection]!

        for key in plotsInSection.keys {
            let plotData = plotsInSection[key]!
            if (
                plotData.left <= left + Int32(width) &&
                plotData.left + Int32(plotData.width) > left + 1 &&
                plotData.top < top + Int32(height) &&
                plotData.top + Int32(plotData.height) > top + 1
            ) {
                return true;
            }
        }

        return false;
    }

    //check if it's colliding in all the neighbour sections
    pub fun isPlotColliding(xSection: Int32, ySection: Int32, left: Int32, top: Int32, width: UInt16, height: UInt16) : Bool {
        if(Necryptolis.isPlotCollidingInSection(xSection: xSection, ySection: ySection, left: left, top: top, width: width, height: height)){
            return true
        }
        if(Necryptolis.isPlotCollidingInSection(xSection: xSection + 1, ySection: ySection, left: left, top: top, width: width, height: height)){
            return true
        }
        if( Necryptolis.isPlotCollidingInSection(xSection: xSection, ySection: ySection + 1, left: left, top: top, width: width, height: height)){
            return true
        }
        if(Necryptolis.isPlotCollidingInSection(xSection: xSection + 1, ySection: ySection + 1, left: left, top: top, width: width, height: height)){
            return true
        }
        if(Necryptolis.isPlotCollidingInSection(xSection: xSection - 1, ySection: ySection, left: left, top: top, width: width, height: height)){
            return true
        }
        if(Necryptolis.isPlotCollidingInSection(xSection: xSection, ySection: ySection - 1, left: left, top: top, width: width, height: height)){
            return true
        }
        if(Necryptolis.isPlotCollidingInSection(xSection: xSection - 1, ySection: ySection - 1, left: left, top: top, width: width, height: height)){
            return true
        }
        if(Necryptolis.isPlotCollidingInSection(xSection: xSection - 1, ySection: ySection + 1, left: left, top: top, width: width, height: height)){
            return true
        }
        if(Necryptolis.isPlotCollidingInSection(xSection: xSection + 1, ySection: ySection - 1, left: left, top: top, width: width, height: height)){
            return true
        }
        
        return false
    }

    // returns the price of the plot
    pub fun getPlotPrice(width: UInt16, height: UInt16, left: Int32, top: Int32): UFix64 {
        return Necryptolis.plotSalesInfo.squarePixelPrice * UFix64(height) * UFix64(width) * UFix64(Necryptolis.getPlotDistanceFactor(left: left, top: top))
    }

    // A struct that holds basic information about the plot
    pub struct PlotData {
            pub let id: UInt64

            pub let left: Int32
            pub let top: Int32        
            pub let width: UInt16
            pub let height: UInt16

            init(left: Int32, top: Int32, width: UInt16, height: UInt16) {
                self.id = Necryptolis.totalSupply + 1
                self.left = left
                self.top = top
                self.width = width
                self.height = height
            }
    }

    // A struct that holds information about candle purchases
    pub struct CandleBuy {
        pub let buyerAddress: Address
        pub let timestamp: UFix64

        init(buyerAddress: Address, timestamp: UFix64) {
            self.buyerAddress = buyerAddress
            self.timestamp = timestamp
        }
    }

    // A struct that holds data about grave which is inserted into a cemetery plot
    pub struct GraveData {
        pub let name: String        
        pub let fromDate: String        
        pub(set) var toDate: String
        pub let dateCreated: UFix64
        pub let metadata: {String:String}

        init(name: String, fromDate: String, toDate: String, metadata: {String : String}) {
            self.metadata = metadata
            self.name = name
            self.fromDate = fromDate
            self.toDate = toDate
            self.dateCreated = getCurrentBlock().timestamp
        }
    }

    // NFT
    // A CemeteryPlot NFT resource
    //
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        // The token's ID
        pub let id: UInt64

        pub var plotData: PlotData
        pub var graveData: GraveData  
        pub var isGraveSet: Bool
        
        pub var buriedNFT: @AnyResource{NonFungibleToken.INFT}?

        pub var candles: [CandleBuy]
        pub var lastTrimTimestamp: UFix64

        // initializer
        init(xSection: Int32, ySection: Int32) {
            Necryptolis.totalSupply = Necryptolis.totalSupply + 1
            self.id = Necryptolis.totalSupply 
            self.plotData = Necryptolis.plotDatas[xSection]![ySection]![self.id]!
            self.graveData = GraveData(name: "", fromDate: "", toDate: "", metadata: {})
            self.isGraveSet = false
            self.candles = []
            self.lastTrimTimestamp = getCurrentBlock().timestamp
            self.buriedNFT <- nil

            emit Minted(id: self.id, typeID: 1)
            emit CemeteryPlotMinted(id: self.id, plotData: self.plotData)
        }


        // addGravestone adds a gravestone to the cemetery plot
        //
        // Parameters: 
        // name: Title displayed on the grave
        // fromDate: Starting date displayed on the grave
        // toDate: Ending date displayed on the grave
        // metadata: Additional details like grave shape, color, font etc.
        pub fun addGravestone(name: String, fromDate: String, toDate: String, metadata: {String:String}) {
            pre {
                self.isGraveSet == false : "Grave must be empty."
                name.length < 120 : "Name must be less than 120 characters long."
                name.length > 0 : "Name must be provided."
                fromDate.length < 20 : "From date must be less than 20 characters long."
                toDate.length < 20 : "To date must be less than 20 characters long."
            }
            self.graveData = GraveData(name: name, fromDate: fromDate, toDate: toDate, metadata: metadata)
            self.isGraveSet = true

            emit GravestoneCreated(id: self.id, name: name, fromDate: fromDate, toDate: toDate, metadata: metadata, left: self.plotData.left, top: self.plotData.top)
        }   

        // setToDate sets the ending date on the grave
        //
        // Parameters: 
        // toDate: Ending date displayed on the grave
        pub fun setToDate(toDate: String) {
            pre {
                self.isGraveSet == true : "Grave must be set"
                self.graveData.toDate == "" : "To date on grave must be empty"
                 toDate.length < 20 : "To date must be less than 20 characters long."
            }
            self.graveData.toDate = toDate
            self.isGraveSet = true            

            emit ToDateSet(id: self.id, toDate: toDate, left: self.plotData.left, top: self.plotData.top)
        }
        
        // lightCandle adds a candle to candles list of the NFT
        //
        // Parameters: 
        // buyerPayment: payment vault of the buyer
        // buyerAddress: the address of user buying the candle
        pub fun lightCandle(buyerPayment: @FungibleToken.Vault, buyerAddress: Address){
            pre {
                buyerPayment.balance == Necryptolis.plotSalesInfo.candlePrice : "Payment does not equal price of the candle."
            }
             
            Necryptolis.plotSalesInfo.servicesProviderVault!.borrow()!.deposit(from: <- buyerPayment)

            self.candles.append(CandleBuy(buyerAddress: buyerAddress, timestamp: getCurrentBlock().timestamp))
               
            emit CandleLit(id: self.id, left: self.plotData.left, top: self.plotData.top, buyerAddress: buyerAddress) 
        }  

        // trim resets the last trimmed timestamp to current timestamp
        //
        // Parameters: 
        // buyerPayment: payment vault of the buyer
        pub fun trim(buyerPayment: @FungibleToken.Vault){
            pre {
                buyerPayment.balance == Necryptolis.plotSalesInfo.trimPrice : "Payment does not equal price of the candle."
            }
            
            Necryptolis.plotSalesInfo.servicesProviderVault!.borrow()!.deposit(from: <- buyerPayment)

            self.lastTrimTimestamp = getCurrentBlock().timestamp
               
            emit CemeteryPlotTrimmed(id: self.id, left: self.plotData.left, top: self.plotData.top) 
        }   

        // bury buries a different NFT inside this NFT
        //
        // Parameters: 
        // buyerPayment: payment vault of the buyer
        pub fun bury(nft: @AnyResource{NonFungibleToken.INFT}, nftType: Type){
            pre {
                self.buriedNFT == nil : "NFT already buried here."
            }
            let nftId = nft.id
            self.buriedNFT <-! nft
            
            emit NFTBuried(id: self.id, left: self.plotData.left, top: self.plotData.top, nftID: nftId, nftType: nftType)
        }

        destroy() {
            destroy self.buriedNFT
        } 

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>(), Type<String>()
            ]
        }

        // returns the name of NFT
        pub fun name(): String {
            if(self.isGraveSet) {
                return "Id: ".concat(self.id.toString())
                        .concat(" - ").concat(self.graveData.name)

            } else {
                return "Id: ".concat(self.id.toString())
                .concat(" Plot : (").concat(self.plotData.left.toString()).concat(",").concat(self.plotData.top.toString()).concat(")")
                .concat(" - ").concat(self.plotData.width.toString()).concat("x").concat(self.plotData.height.toString())            }
        }

        // returns additional details about NFT
        pub fun description(): String {
            if(self.isGraveSet) {
                return "Id: ".concat(self.id.toString())
                        .concat(" Title: ").concat(self.graveData.name)
                        .concat(" From: ").concat(self.graveData.fromDate)
                        .concat(" To: ").concat(self.graveData.toDate)
            } else {
                return "Id: ".concat(self.id.toString())
                .concat(" Position: (").concat(self.plotData.left.toString()).concat(",").concat(self.plotData.top.toString()).concat(")")
                .concat(" Dimensions: ").concat(self.plotData.width.toString()).concat("x").concat(self.plotData.height.toString())            }
        }

        // imageURL returns the public URL of a picture of this NFT
        pub fun imageURL(): String {
            return Necryptolis.imagesBaseURL.concat(self.id.toString()).concat(".png")
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: self.name(),
                        description: self.description(),
                        thumbnail: MetadataViews.HTTPFile(
                            url: self.imageURL()
                            )
                    )
                case Type<String>():
                    return self.name()
            }

            return nil
        }
    }


    // A resource that a user who wishes to mint NecryptolisNFTs holds
    pub resource PlotMinter {
        
        // mintCemeteryPlot mints a Necryptolis cemetery plot NFT
        //
        // Parameters: 
        // left: distance from the center to the left
        // top: distance from the center to the top
        // width: The width of the plot
        // height: The height of the plot
        // buyerPayment: payment vault of the buyer
        pub fun mintCemeteryPlot(left: Int32, top: Int32, width: UInt16, height: UInt16, buyerPayment: @FungibleToken.Vault): @NFT {   
            pre {
                !Necryptolis.isPlotColliding(xSection: Necryptolis.getSection(position: left), ySection: Necryptolis.getSection(position: top),left: left, top: top, width: width, height: height) : "New plot is colliding with the old."                                
                buyerPayment.balance == Necryptolis.getPlotPrice(width: width, height: height, left: left, top: top) : "Payment does not equal the price of the plot."
                width <= Necryptolis.plotSalesInfo.maxPlotWidth : "Plot too wide."
                width >= Necryptolis.plotSalesInfo.minPlotWidth : "Plot not wide enough."
                height <= Necryptolis.plotSalesInfo.maxPlotHeight : "Plot too high."
                height >= Necryptolis.plotSalesInfo.minPlotHeight : "Plot not high enough."
            }
                
            var newPlotData = PlotData(left: left, top: top, width: width, height: height)
            let xSection = Necryptolis.getSection(position: left)         
            let ySection = Necryptolis.getSection(position: top)
            if(Necryptolis.plotDatas[xSection] == nil){        
                Necryptolis.plotDatas[xSection] = {}
            }
            let plotDatasYSection = Necryptolis.plotDatas[xSection]!
            
            if(plotDatasYSection[ySection] == nil){
                plotDatasYSection[ySection] = {}
            }
            
            let plotsInSection = plotDatasYSection[ySection]!
            plotsInSection[newPlotData.id] = newPlotData
            plotDatasYSection[ySection] = plotsInSection
            Necryptolis.plotDatas[xSection] = plotDatasYSection
            
            Necryptolis.plotSalesInfo.servicesProviderVault!.borrow()!.deposit(from: <- buyerPayment)
            
            //Mint
            let newCemeteryPlot: @NFT <- create NFT(xSection: xSection, ySection: ySection)

            return <- newCemeteryPlot
        }
    }

    // Admin resource which only the admin holds 
    // and is able to change plotSalesInfo, mint NFTs, 
    // change base images url and create new admins
    pub resource Admin {

        // changePlotSalesInfo values inside the plotSalesInfo property of the contract
        //
        // Parameters: 
        // squarePixelPrice: price per pixel of a cemetery plot
        // candlePrice: price of a candle being added to a plot
        // trimPrice: price of trimming the plot
        // maxPlotHeight: The maximum allowed height of the plot
        // maxPlotWidth: The maximum allowed width of the plot
        // minPlotHeight: The minimum allowed height of the plot
        // minPlotWidth: The minimum allowed height of the plot
        // vault: the vault which will receive all the tokens
        //
        // Restrictions:
        // prices and minimum values must be positive numbers
        // maximum values must be greater than minimum values
        pub fun changePlotSalesInfo(squarePixelPrice: UFix64, candlePrice: UFix64, trimPrice: UFix64, maxPlotHeight: UInt16, maxPlotWidth: UInt16, minPlotHeight: UInt16, minPlotWidth: UInt16, vault: Capability<&{FungibleToken.Receiver}>){
            pre {
                squarePixelPrice > 0.0 : "Square pixel price must be greater than 0."
                candlePrice > 0.0 : "Candle price must be greater than 0."
                trimPrice > 0.0 : "Trimming price must be greater than 0."                
                minPlotHeight > 0 : "Min height must be greater than 0."
                minPlotWidth > 0 : "Min Width must be greater than 0."
                maxPlotHeight > minPlotHeight : "Max Height must be greater than minPlotHeight."
                maxPlotWidth > minPlotWidth : "Max Width must be greater than minPlotWidth."
            }

            Necryptolis.plotSalesInfo.squarePixelPrice = squarePixelPrice
            Necryptolis.plotSalesInfo.candlePrice = candlePrice
            Necryptolis.plotSalesInfo.trimPrice = trimPrice
            Necryptolis.plotSalesInfo.maxPlotHeight = maxPlotHeight
            Necryptolis.plotSalesInfo.maxPlotWidth = maxPlotWidth
            Necryptolis.plotSalesInfo.minPlotHeight = minPlotHeight
            Necryptolis.plotSalesInfo.minPlotWidth = minPlotWidth
            Necryptolis.plotSalesInfo.servicesProviderVault = vault
            
            Necryptolis.PlotSalesInfo(
                squarePixelPrice: squarePixelPrice, 
                candlePrice: candlePrice, 
                trimPrice: trimPrice, 
                maxPlotHeight: maxPlotHeight,
                maxPlotWidth: maxPlotWidth, 
                minPlotHeight: minPlotHeight, 
                minPlotWidth: minPlotWidth,
                vault: vault
            )
             
            emit PlotSalesInfoChanged(squarePixelPrice: squarePixelPrice, candlePrice: candlePrice, trimPrice: trimPrice, maxPlotHeight: maxPlotHeight, maxPlotWidth: maxPlotWidth, minPlotHeight: minPlotHeight, minPlotWidth: minPlotWidth, vaultAddress: vault.address, vaultType: vault.getType())       
        }


        // mintCemeteryPlot mints a Necryptolis cemetery plot NFT
        //
        // Parameters: 
        // left: distance from the center to the left
        // top: distance from the center to the top
        // width: The width of the plot
        // height: The height of the plot
        pub fun mintCemeteryPlot(left: Int32, top: Int32, width: UInt16, height: UInt16): @NFT {  
            pre {
                !Necryptolis.isPlotColliding(xSection: Necryptolis.getSection(position: left), ySection: Necryptolis.getSection(position: top),left: left, top: top, width: width, height: height) : "New plot is colliding with the old."                               
            }         
            var newPlotData = PlotData(left: left, top: top, width: width, height: height)  
            let xSection = Necryptolis.getSection(position: left)         
            let ySection = Necryptolis.getSection(position: top)
            if(Necryptolis.plotDatas[xSection] == nil){        
                Necryptolis.plotDatas[xSection] = {}
            }
            let plotDatasYSection = Necryptolis.plotDatas[xSection]!
            
            if(plotDatasYSection[ySection] == nil){
                plotDatasYSection[ySection] = {}
            }
            
            let plotsInSection = plotDatasYSection[ySection]!
            plotsInSection[newPlotData.id] = newPlotData
            plotDatasYSection[ySection] = plotsInSection
            Necryptolis.plotDatas[xSection] = plotDatasYSection 
            
            //Mint
            let newCemeteryPlot: @NFT <- create NFT(xSection: xSection, ySection: ySection)

            return <- newCemeteryPlot
        }

        // changeImagesBaseUrl mints a Necryptolis cemetery plot NFT
        //
        // Parameters: 
        // baseUrl: the url which will be used to append the id and extension of NFT image (${imagesBaseURL}/1.png    
        pub fun changeImagesBaseUrl(baseUrl: String) {
            Necryptolis.imagesBaseURL = baseUrl
        }

        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    pub fun createPlotMinter(): @PlotMinter {
        return <- create PlotMinter()
    }

    pub fun createGravestoneManager(): @GravestoneManager {
        return <- create GravestoneManager()
    }

    // The standard public collection interface
    // to allow users to deposit/borrow Necryptolis NFTs
    // but also to light a candle and trim them
    pub resource interface NecryptolisCollectionPublic {        
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]        
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowCemeteryPlot(id: UInt64): &Necryptolis.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Necryptolis reference: The ID of the returned reference is incorrect"
            }
        }
        pub fun lightCandle(cemeteryPlotId: UInt64, buyerPayment: @FungibleToken.Vault, buyerAddress: Address)        
        pub fun trimCemeteryPlot(cemeteryPlotId: UInt64, buyerPayment: @FungibleToken.Vault)
    }

    pub resource Collection: NecryptolisCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}


        // withdraw removes an NFT from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a Necryptolis NFT and adds it to the Collections dictionary
        //
        // Parameters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Necryptolis.NFT

            let id: UInt64 = token.id
            let left: Int32 = token.plotData.left
            let top: Int32 = token.plotData.top

            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)
            emit DepositNecryptolisNFT(id: id, to: self.owner?.address, left: left, top: top)

            destroy oldToken
        }

        // gets Ids of all the nfts owned by this collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowCemeteryPlot(id: UInt64): &Necryptolis.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Necryptolis.NFT
            } else {
                return nil
            }
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let necryptolisNft = nft as! &Necryptolis.NFT
            return necryptolisNft as &AnyResource{MetadataViews.Resolver}
        }


        // lightCandle adds a candle to candles list of the NFT
        //
        // Parameters: 
        // cemeteryPlotId: id of the NFT
        // buyerPayment: payment vault of the buyer
        // buyerAddress: the address of user buying the candle
        pub fun lightCandle(cemeteryPlotId: UInt64, buyerPayment: @FungibleToken.Vault, buyerAddress: Address) {
            pre {
                self.ownedNFTs[cemeteryPlotId] != nil : "Cemetery plot not in the collection."
            }
           
            var cemeteryPlot = self.borrowCemeteryPlot(id: cemeteryPlotId)!
            cemeteryPlot.lightCandle(buyerPayment: <- buyerPayment, buyerAddress: buyerAddress)
        }

        
        // trimCemeteryPlot trims the plot and resets lastTrimDate to current block timestamp
        //
        // Parameters: 
        // cemeteryPlotId: id of the NFT
        // buyerPayment: payment vault of the buyer
        // buyerAddress: the address of user buying the candle
        pub fun trimCemeteryPlot(cemeteryPlotId: UInt64, buyerPayment: @FungibleToken.Vault) {
            pre {
                self.ownedNFTs[cemeteryPlotId] != nil : "Cemetery plot not in the collection."
            }
           
            var cemeteryPlot = self.borrowCemeteryPlot(id: cemeteryPlotId)!
            cemeteryPlot.trim(buyerPayment: <- buyerPayment)
        }

        destroy() {
            destroy self.ownedNFTs
        }

        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // public function that anyone can call to create a new empty collection
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub resource interface GravestoneCreator {        
        // Allows the Cemetery plot owner to create a gravestone
        pub fun createGravestone(
            necryptolisProviderCapability: Capability<&Necryptolis.Collection{NecryptolisCollectionPublic}>,            
            nftID: UInt64,
            graveData: GraveData
        )
    }

    pub resource interface BurialProvider {        
        // Allows the Cemetery plot owner to bury an NFT
        //
        pub fun buryNFT(
            necryptolisProviderCapability: Capability<&Necryptolis.Collection{NecryptolisCollectionPublic}>,
            plotID: UInt64,            
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64
        )
    }

    // A resource which a user who wishes to add gravestones or 
    // bury other NFTs into his Necryptolis NFT holds
    //
    pub resource GravestoneManager : GravestoneCreator, BurialProvider {
        // createGravestone creates a gravestone on cemetery plot
        // Parameters: 
        // necryptolisProviderCapability: provider capability for the nfts
        // nftID: id of the NFT for which the gravestone is being created
        // graveData: the basic data and metadata of the grave
         pub fun createGravestone(
            necryptolisProviderCapability: Capability<&Necryptolis.Collection{NecryptolisCollectionPublic}>,
            nftID: UInt64,
            graveData: GraveData                     
         ) {

            let provider = necryptolisProviderCapability.borrow()
            assert(provider != nil, message: "cannot borrow necryptolisProviderCapability")

            let nft = provider!.borrowCemeteryPlot(id: nftID)!

            nft.addGravestone(name: graveData.name, fromDate: graveData.fromDate, toDate: graveData.toDate, metadata: graveData.metadata)
        }

        // buryNFT is used to bury a different NFT into a Necryptolis NFT plot
        //
        // Parameters: 
        // necryptolisProviderCapability: provider capability for the necryptolis nfts
        // plotID: id of the NFT in which the other NFT is being buried
        // nftProviderCapability: provider capability for the nft which is being buried
        // nftType: the type of the nft being buried
        // nftId: the id of the nft being buried
        //
        // restrictions: nft mustn't be Necryptolis NFT
        pub fun buryNFT(
            necryptolisProviderCapability: Capability<&Necryptolis.Collection{NecryptolisCollectionPublic}>,
            plotID: UInt64,            
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64
         ) {
            pre {                
                nftType != Necryptolis.NFT.getType()
            }
            let provider = necryptolisProviderCapability.borrow()
            assert(provider != nil, message: "cannot borrow necryptolisProviderCapability")
            let plotNFT = provider!.borrowCemeteryPlot(id: plotID)!
            
            let nftProvider = nftProviderCapability.borrow()
            assert(nftProvider != nil, message: "cannot borrow nftProviderCapability")
            let nft <- nftProvider!.withdraw(withdrawID: nftID)
            assert(nft.isInstance(nftType), message: "token is not of specified type")

            plotNFT.bury(nft: <- nft, nftType: nftType)
        }
        

        init () {
            
        }
    }

  init() {
      self.totalSupply = 0
      self.plotDatas = {}   
      self.plotSalesInfo = PlotSalesInfo(squarePixelPrice: 0.001, candlePrice: 1.0, trimPrice: 1.0, maxPlotHeight: 400, maxPlotWidth: 400, minPlotHeight: 200, minPlotWidth: 200, vault: nil)
      self.imagesBaseURL = "https://necryptolis.azureedge.net/images/"

      //Initialize storage paths
      self.CollectionStoragePath = /storage/NecryptolisCollection
      self.CollectionPublicPath = /public/NecryptolisCollection
      self.ResolverCollectionPublicPath = /public/NecryptolisResolverCollection
      self.NecryptolisAdminStoragePath = /storage/NecryptolisAdmin
      self.GravestoneManagerStoragePath = /storage/GravestoneManager
      self.PlotMinterStoragePath = /storage/NecryptolisPlotMinter

      self.account.save(<- create Admin(), to: self.NecryptolisAdminStoragePath)
    
      // Collection
      self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)
      self.account.link<&{NecryptolisCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)
      self.account.link<&{MetadataViews.ResolverCollection}>(self.ResolverCollectionPublicPath, target: self.CollectionStoragePath)  
  }
}
