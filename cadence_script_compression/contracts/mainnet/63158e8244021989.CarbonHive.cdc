import NonFungibleToken from 0x1d7e57aa55817448

pub contract CarbonHive: NonFungibleToken {

    pub event ContractInitialized()
    pub event FundingRoundCreated(id: UInt32, projectID: UInt32, name: String)
    pub event ReportCreated(id: UInt32, projectID: UInt32)
    pub event ProjectCreated(id: UInt32, name: String)
    pub event FundingRoundAddedToProject(projectID: UInt32, fundingRoundID: UInt32)
    pub event ReportAddedToProject(projectID: UInt32, reportID: UInt32)
    pub event ReportAddedToFundingRound(projectID: UInt32, fundingRoundID: UInt32, reportID: UInt32)
    pub event CompletedFundingRound(projectID: UInt32, fundingRoundID: UInt32, numImpacts: UInt32)
    pub event ProjectClosed(projectID: UInt32)
    pub event ImpactMinted(
        impactID: UInt64,
        projectID: UInt32,
        fundingRoundID: UInt32,
        serialNumber: UInt32,
        amount: UInt32,
        location: String,
        locationDescriptor: String,
        vintagePeriod: String
    )
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event ImpactDestroyed(id: UInt64)


    access(self) var projectDatas: {UInt32: ProjectData}
    access(self) var fundingRoundDatas: {UInt32: FundingRoundData}
    access(self) var reportDatas: {UInt32: ReportData}
    access(self) var projects: @{UInt32: Project}

    pub let ImpactCollectionStoragePath: StoragePath
    pub let ImpactCollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath
    
    pub var nextFundingRoundID: UInt32
    pub var nextProjectID: UInt32
    pub var nextReportID: UInt32
    pub var totalSupply: UInt64

    pub struct FundingRoundData {
        pub let fundingRoundID: UInt32
        pub let name: String
        pub let description: String
        pub let formula: String
        pub let formulaType: String
        pub let unit: String
        pub let vintagePeriod: String
        pub let totalAmount: UInt32
        pub let roundEnds: Fix64
        pub let location: String
        pub let locationDescriptor: String
        pub let projectID: UInt32
        access(self) let reports: [UInt32]
        access(self) let metadata: {String: String}

        init(
            name: String,
            description: String,
            formula: String,
            formulaType: String,
            unit: String,
            vintagePeriod: String,
            totalAmount: UInt32,
            roundEnds: Fix64,
            location: String,
            locationDescriptor: String,
            projectID: UInt32,
            metadata: {String: String}
        ) {
            pre {
                name != "": "New fundingRound name cannot be empty"
            }
            self.fundingRoundID = CarbonHive.nextFundingRoundID
            self.name = name
            self.description = description
            self.formula = formula
            self.formulaType = formulaType
            self.unit = unit
            self.vintagePeriod = vintagePeriod
            self.totalAmount = totalAmount
            self.roundEnds = roundEnds
            self.location = location
            self.locationDescriptor = locationDescriptor
            self.projectID = projectID
            self.reports = []
            self.metadata = metadata
            CarbonHive.nextFundingRoundID = CarbonHive.nextFundingRoundID + (1 as UInt32)
            emit FundingRoundCreated(id: self.fundingRoundID, projectID: projectID, name: name)
        }

        pub fun addReport(reportID: UInt32) {
            self.reports.append(reportID)
        }
    }

    pub struct ProjectData {
        pub let projectID: UInt32
        pub var closed: Bool
        pub let url: String
        pub let name: String
        pub let developer: String
        pub let description: String
        pub let location: String
        pub let locationDescriptor: String
        pub let type: String
        access(self) let metadata: {String: String}

        pub fun close() {
            self.closed = true
        }

        init(
            closed: Bool,
            url: String,
            metadata: { String: String},
            name: String,
            developer: String,
            description: String,
            location: String,
            locationDescriptor: String,
            type: String,
        ) {
            pre {
                name != "": "New project name cannot be empty"
            }
            self.projectID = CarbonHive.nextProjectID
            self.closed = closed
            self.url = url
            self.metadata = metadata
            self.name = name
            self.developer = developer
            self.description = description
            self.location = location
            self.locationDescriptor = locationDescriptor
            self.type = type

            CarbonHive.nextProjectID = CarbonHive.nextProjectID + (1 as UInt32)

            emit ProjectCreated(id: self.projectID, name: name)
        }
    }

    pub struct ReportData {
        pub let reportID: UInt32
        pub let date: String
        pub let projectID: UInt32
        pub let fundingRoundID: UInt32
        pub let description: String
        pub let reportContent: String
        pub let reportContentType: String
        access(self) let metadata: {String: String}

        init(
            date: String,
            projectID: UInt32,
            fundingRoundID: UInt32,
            description: String,
            reportContent: String,
            reportContentType: String,
            metadata: {String: String}
        ) {
            self.reportID = CarbonHive.nextReportID
            self.date = date
            self.projectID = projectID
            self.fundingRoundID = fundingRoundID
            self.description = description
            self.reportContent = reportContent
            self.reportContentType = reportContentType
            self.metadata = metadata
            CarbonHive.nextReportID = CarbonHive.nextReportID + (1 as UInt32)
            emit ReportCreated(id: self.reportID, projectID: projectID)
        }
    }


    // Admin can call the Project resoure's methods to add FundingRound,
    // add Report and Mint Impact.
    //
    // Project can have zero to many FundingRounds and Reports.
    //
    // Impact NFT belogs to the project that minted it, and references the actual FundingRound
    // the Impact was minted for.
    pub resource Project {
        pub let projectID: UInt32
        access(self) var fundingRounds: [UInt32]
        access(self) var fundingRoundCompleted: {UInt32: Bool}
        pub var closed: Bool
        pub let url: String
        access(self) var metadata: {String: String}
        pub let name: String
        pub let developer: String
        pub let description: String
        pub let location: String
        pub let locationDescriptor: String
        pub let type: String
        access(self) var reports: [UInt32]
        access(self) var impactMintedPerFundingRound: {UInt32: UInt32}
        access(self) var impactAmountPerFundingRound: {UInt32: UInt32}

        pub fun getFundingRounds(): [UInt32] {
            return self.fundingRounds
        }

        pub fun getFundingRoundCompleted(fundingRoundID: UInt32): Bool? {
            return self.fundingRoundCompleted[fundingRoundID]
        }

        pub fun getReports(): [UInt32] {
            return self.reports
        } 

        pub fun getImpactMintedPerFundingRound(fundingRoundID: UInt32): UInt32? {
            return self.impactMintedPerFundingRound[fundingRoundID]
        }

        pub fun getImpactAmountPerFundingRound(fundingRoundID: UInt32): UInt32? {
            return self.impactAmountPerFundingRound[fundingRoundID]
        }

        pub fun getMetadata(): {String: String} {
            return self.metadata
        } 
        
        init(
            name: String,
            description: String,
            url: String,
            developer: String,
            type: String,
            location: String,
            locationDescriptor: String,
            metadata: {String: String}
        ) {
            self.projectID = CarbonHive.nextProjectID
            self.fundingRounds = []
            self.reports = []
            self.fundingRoundCompleted = {}
            self.closed = false
            self.url = url
            self.impactMintedPerFundingRound = {}
            self.impactAmountPerFundingRound = {}
            self.metadata = metadata
            self.type = type
            self.name = name
            self.description = description
            self.developer = developer
            self.location = location
            self.locationDescriptor = locationDescriptor

            CarbonHive.projectDatas[self.projectID] = ProjectData(
                closed: self.closed,
                url: url,
                metadata: metadata,
                name: name,
                developer: developer,
                description: description,
                location: location,
                locationDescriptor: locationDescriptor,
                type: type,
            )
        }

        pub fun addReport(reportID: UInt32) {
            pre {
                CarbonHive.reportDatas[reportID] != nil: "Cannot add the Report to Project: Report doesn't exist."
            }
            self.reports.append(reportID)
            emit ReportAddedToProject(projectID: self.projectID, reportID: reportID)
        }

        // Add report to both Funding Round and owning Project
        pub fun addReportToFundingRound(reportID: UInt32, fundingRoundID: UInt32) {
            pre {
                CarbonHive.reportDatas[reportID] != nil: "Cannot add the Report to Funding Round: Report doesn't exist."
                CarbonHive.fundingRoundDatas[fundingRoundID] != nil: "Cannot add the Report to Funding Round: Funding Round doesn't exist."
            }
            let fundingRound = CarbonHive.fundingRoundDatas[fundingRoundID]!
            self.reports.append(reportID)
            fundingRound.addReport(reportID: reportID)
            CarbonHive.fundingRoundDatas[fundingRoundID] = fundingRound
            emit ReportAddedToFundingRound(projectID: self.projectID, fundingRoundID: fundingRoundID, reportID: reportID)
        }

        pub fun addFundingRound(fundingRoundID: UInt32) {
            pre {
                CarbonHive.fundingRoundDatas[fundingRoundID] != nil: "Cannot add the FundingRound to Project: FundingRound doesn't exist."
                !self.closed: "Cannot add the FundingRound to the Project after the Project has been closed."
                self.impactMintedPerFundingRound[fundingRoundID] == nil: "The FundingRound has already beed added to the Project."
            }

            self.fundingRounds.append(fundingRoundID)
            self.fundingRoundCompleted[fundingRoundID] = false
            self.impactMintedPerFundingRound[fundingRoundID] = 0
            self.impactAmountPerFundingRound[fundingRoundID] = 0
            emit FundingRoundAddedToProject(projectID: self.projectID, fundingRoundID: fundingRoundID)
        }

        pub fun completeFundingRound(fundingRoundID: UInt32) {
            pre {
                self.fundingRoundCompleted[fundingRoundID] != nil: "Cannot complete the FundingRound: FundingRound doesn't exist in this Project!"
            }

            if !self.fundingRoundCompleted[fundingRoundID]! {
                self.fundingRoundCompleted[fundingRoundID] = true

                emit CompletedFundingRound(
                    projectID: self.projectID,
                    fundingRoundID: fundingRoundID,
                    numImpacts: self.impactMintedPerFundingRound[fundingRoundID]!
                )
            }
        }

        pub fun completeAllFundingRound() {
            for fundingRound in self.fundingRounds {
                self.completeFundingRound(fundingRoundID: fundingRound)
            }
        }

        pub fun close() {
            if !self.closed {
                self.closed = true
                let projectData = CarbonHive.projectDatas[self.projectID] ?? panic("Could not finf project data")
                projectData.close()
                CarbonHive.projectDatas[self.projectID] = projectData
                emit ProjectClosed(projectID: self.projectID)
            }
        }

        pub fun mintImpact(
            fundingRoundID: UInt32,
            amount: UInt32,
            location: String,
            locationDescriptor: String,
            vintagePeriod: String,
            content: @AnyResource{CarbonHive.Content}
        ): @NFT {
            pre {
                CarbonHive.fundingRoundDatas[fundingRoundID] != nil: "Cannot mint the Impact: This FundingRound doesn't exist."
                !self.fundingRoundCompleted[fundingRoundID]!: "Cannot mint the Impact from this FundingRound: This FundingRound has been completed."
            }

            let block = getCurrentBlock()
            let time = Fix64(block.timestamp)
            let fundingRound = CarbonHive.fundingRoundDatas[fundingRoundID]!
            if fundingRound.roundEnds < time {
                panic("The funding round ended on ".concat(fundingRound.roundEnds.toString()).concat(" now: ").concat(block.timestamp.toString()))
            }

            let amountInFundingRound = self.impactAmountPerFundingRound[fundingRoundID]!
            let remainingAmount = fundingRound.totalAmount - amountInFundingRound
            if amount >= remainingAmount {
                panic("Not enough amount left for minting impact: ".concat(amountInFundingRound.toString()).concat(" amount minted, ").concat(remainingAmount.toString()).concat(" amount remaining."))
            }

            let impactsInFundingRound = self.impactMintedPerFundingRound[fundingRoundID]!

            let newImpact: @NFT <- create NFT(
                projectID: self.projectID,
                fundingRoundID: fundingRoundID,
                serialNumber: impactsInFundingRound + (1 as UInt32),
                amount: amount, 
                location: location,
                locationDescriptor: locationDescriptor,
                vintagePeriod: vintagePeriod,
                content: <- content
            )

            self.impactMintedPerFundingRound[fundingRoundID] = impactsInFundingRound + (1 as UInt32)
            self.impactAmountPerFundingRound[fundingRoundID] = amountInFundingRound + amount
            return <-newImpact
        }
    }


    pub struct ImpactData {
        pub let projectID: UInt32
        pub let fundingRoundID: UInt32
        pub let serialNumber: UInt32
        pub let amount: UInt32
        pub let location: String
        pub let locationDescriptor: String
        pub let vintagePeriod: String

        init(
            projectID: UInt32,
            fundingRoundID: UInt32,
            serialNumber: UInt32,
            amount: UInt32,
            location: String,
            locationDescriptor: String,
            vintagePeriod: String
        ) {
            self.projectID = projectID
            self.fundingRoundID = fundingRoundID
            self.serialNumber = serialNumber
            self.amount = amount
            self.location = location
            self.locationDescriptor = locationDescriptor
            self.vintagePeriod = vintagePeriod
        }
    }

    pub resource interface Content {
        pub fun getData(): String
        pub fun getContentType(): String
    }

    pub resource ImpactContent: Content {
        access(contract) let data: String
        access(contract) let type: String

        init(data: String, type: String) {
            self.data = data
            self.type = type
        }

        pub fun getData(): String {
            return self.data
        }

        pub fun getContentType(): String {
            return self.type
        }
    }


    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64 
        pub let data: ImpactData
        access(self) let content: @AnyResource{CarbonHive.Content}

        pub fun getContentData() : String {
            return self.content.getData()
        }

        pub fun getContentType() : String {
            return self.content.getContentType()
        }

        init(
            projectID: UInt32,
            fundingRoundID: UInt32,
            serialNumber: UInt32,
            amount: UInt32,
            location: String,
            locationDescriptor: String,
            vintagePeriod: String,
            content: @AnyResource{CarbonHive.Content}
        ) {
            CarbonHive.totalSupply = CarbonHive.totalSupply + (1 as UInt64)
            self.id = CarbonHive.totalSupply
            self.data = ImpactData(
                projectID: projectID,
                fundingRoundID: fundingRoundID,
                serialNumber: serialNumber,
                amount: amount,
                location: location,
                locationDescriptor: locationDescriptor,
                vintagePeriod: vintagePeriod
            )
            self.content <- content 

            emit ImpactMinted(
                impactID: self.id,
                projectID: self.data.projectID,
                fundingRoundID: self.data.fundingRoundID,
                serialNumber: self.data.serialNumber,
                amount: self.data.amount,
                location: self.data.location,
                locationDescriptor: self.data.locationDescriptor,
                vintagePeriod: self.data.vintagePeriod
            )
        }

        destroy() {
            destroy self.content
            emit ImpactDestroyed(id: self.id)
        }
    }

    pub resource interface ProjectAdmin {

        pub fun createFundingRound(
            name: String,
            description: String,
            formula: String,
            formulaType: String,
            unit: String,
            vintagePeriod: String,
            totalAmount: UInt32,
            roundEnds: Fix64,
            location: String,
            locationDescriptor: String,
            projectID: UInt32,
            metadata: {String: String}
        ): UInt32

        pub fun createReport(
            date: String,
            projectID: UInt32,
            fundingRoundID: UInt32,
            description: String,
            reportContent: String,
            reportContentType: String,
            metadata: {String: String}
        ): UInt32

        pub fun createProject(
            name: String, 
            description: String, 
            url: String,
            developer: String, 
            type: String, 
            location: String, 
            locationDescriptor: String, 
            metadata: {String: String}
        )

        pub fun borrowProject(projectID: UInt32): &Project

    }

    pub resource interface ContentAdmin {
        pub fun createContent(
            data: String,
            contentType: String
        ): @AnyResource{CarbonHive.Content}
    }

    pub resource Admin: ProjectAdmin, ContentAdmin {

        pub fun createFundingRound(
            name: String,
            description: String,
            formula: String,
            formulaType: String,
            unit: String,
            vintagePeriod: String,
            totalAmount: UInt32,
            roundEnds: Fix64,
            location: String,
            locationDescriptor: String,
            projectID: UInt32,
            metadata: {String: String}
        ): UInt32 {
            var newFundingRound = FundingRoundData(
                name: name,
                description: description,
                formula: formula,
                formulaType: formulaType,
                unit: unit,
                vintagePeriod: vintagePeriod,
                totalAmount: totalAmount,
                roundEnds: roundEnds,
                location: location,
                locationDescriptor: locationDescriptor,
                projectID: projectID,
                metadata: metadata
            )
            let newID = newFundingRound.fundingRoundID
            CarbonHive.fundingRoundDatas[newID] = newFundingRound
            return newID
        }

        pub fun createReport(
            date: String,
            projectID: UInt32,
            fundingRoundID: UInt32,
            description: String,
            reportContent: String,
            reportContentType: String,
            metadata: {String: String}
        ): UInt32 {
            var newReport = ReportData(
                date: date,
                projectID: projectID,
                fundingRoundID: fundingRoundID,
                description: description,
                reportContent: reportContent,
                reportContentType: reportContentType,
                metadata: metadata
            )
            let newID = newReport.reportID
            CarbonHive.reportDatas[newID] = newReport
            return newID
        }

        pub fun createProject(
            name: String, 
            description: String, 
            url: String,
            developer: String, 
            type: String, 
            location: String, 
            locationDescriptor: String, 
            metadata: {String: String}
        ) {
            var newProject <- create Project(
                name: name,
                description: description,
                url: url,
                developer: developer,
                type: type,
                location: location,
                locationDescriptor: locationDescriptor,
                metadata: metadata
            )
            CarbonHive.projects[newProject.projectID] <-! newProject
        }

        pub fun createContent(
            data: String,
            contentType: String
        ): @AnyResource{CarbonHive.Content} {
            return <- create ImpactContent(
                data: data,
                type: contentType
            )
        }

        pub fun borrowProject(projectID: UInt32): &Project {
            pre {
                CarbonHive.projects[projectID] != nil: "Cannot borrow Project: The Project doesn't exist"
            }
            return &CarbonHive.projects[projectID] as &Project
        }

        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    pub resource interface ImpactCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowImpact(id: UInt64): &CarbonHive.NFT? {
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Impact reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: ImpactCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 

        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Impact does not exist in the collection")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            return <-batchCollection
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @CarbonHive.NFT
            let id = token.id
            let oldToken <- self.ownedNFTs[id] <- token
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }
            destroy oldToken
        }

        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            let keys = tokens.getIDs()
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }
            destroy tokens
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowImpact(id: UInt64): &CarbonHive.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &CarbonHive.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create CarbonHive.Collection()
    }

    pub fun getProjectData(projectID: UInt32): ProjectData? {
        return self.projectDatas[projectID]
    }

    pub fun getReportData(reportID: UInt32): ReportData? {
        return self.reportDatas[reportID]
    }

    pub fun getFundingRoundData(fundingRoundID: UInt32): FundingRoundData? {
        return self.fundingRoundDatas[fundingRoundID]
    }

    pub fun getFundingRoundsInProject(projectID: UInt32): [UInt32]? {
        return CarbonHive.projects[projectID]?.getFundingRounds()
    }

    pub fun getReportsInProject(projectID: UInt32): [UInt32]? {
        return CarbonHive.projects[projectID]?.getReports()
    }

    pub fun getAmountUsedInFundingRound(projectID: UInt32, fundingRoundID: UInt32): UInt32? {
        if let projectToRead <- CarbonHive.projects.remove(key: projectID) {
            let amount = projectToRead.getImpactAmountPerFundingRound(fundingRoundID: fundingRoundID)
            CarbonHive.projects[projectID] <-! projectToRead
            return amount
        } else {
            // If the project wasn't found return nil
            return nil
        }
    }

    init() {
        self.ImpactCollectionPublicPath = /public/ImpactCollection
        self.ImpactCollectionStoragePath = /storage/ImpactCollection
        self.AdminStoragePath = /storage/CarbonHiveAdmin

        self.projectDatas = {}
        self.fundingRoundDatas = {}
        self.reportDatas = {}
        self.projects <- {}

        self.totalSupply = 0
        self.nextFundingRoundID = 1
        self.nextProjectID = 1
        self.nextReportID = 1

        self.account.save<@Collection>(<- create Collection(), to: self.ImpactCollectionStoragePath)
        self.account.link<&{ImpactCollectionPublic}>(self.ImpactCollectionPublicPath, target: self.ImpactCollectionStoragePath)
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
