import NonFungibleToken from 0x1d7e57aa55817448
import NowggNFT from 0x85b8bbf926dcddfa


pub contract NowggPuzzle {

    pub event ContractInitialized()
    pub event PuzzleRegistered(puzzleId: String, parentNftTypeId: String, childNftTypeIds: [String])
    pub event PuzzleCombined(puzzleId: String, by: Address)

    pub let PuzzleHelperStoragePath: StoragePath
    pub let PuzzleHelperPublicPath: PublicPath

    pub struct Puzzle {
        access(contract) let puzzleId: String
        access(contract) let parentNftTypeId: String
        access(contract) let childNftTypeIds: [String]

        init(puzzleId: String, parentNftTypeId: String, childNftTypeIds: [String]) {
            if (NowggPuzzle.allPuzzles.keys.contains(puzzleId)) {
                panic("Puzzle is already registered")
            }
            if (childNftTypeIds.length >= 1000) {
                panic("Puzzle cannot have more than 1000 pieces")
            }
            self.puzzleId = puzzleId
            self.parentNftTypeId = parentNftTypeId
            self.childNftTypeIds = childNftTypeIds
        }

        pub fun getPuzzleInfo(): {String: AnyStruct} {
            return {
                "puzzleId": self.puzzleId,
                "parentNftTypeId": self.parentNftTypeId,
                "childNftTypeIds": self.childNftTypeIds
            }
        }
    }

    access(contract) var allPuzzles: {String: Puzzle}

    pub resource interface PuzzleHelperPublic {
        pub fun borrowPuzzle(puzzleId: String): Puzzle? {
            post {
                (result == nil) || (result?.puzzleId == puzzleId):
                    "Cannot borrow puzzle reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource interface PuzzleHelperInterface {
        pub fun registerPuzzle(
            nftMinter: &NowggNFT.NFTMinter,
            puzzleId: String,
            parentNftTypeId: String,
            childNftTypeIds: [String],
            maxCount: UInt64,
        )
        pub fun combinePuzzle(
            nftMinter: &NowggNFT.NFTMinter,
            nftProvider: &{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic, NowggNFT.NowggNFTCollectionPublic},
            puzzleId: String,
            parentNftTypeId: String,
            childNftIds: [UInt64],
            metadata: {String: AnyStruct}
        )
    }

    // Resource that allows other accounts to access the functionality related to puzzles
    pub resource PuzzleHelper: PuzzleHelperPublic, PuzzleHelperInterface {

        pub fun borrowPuzzle(puzzleId: String): Puzzle? {
            if NowggPuzzle.allPuzzles[puzzleId] != nil {
                return NowggPuzzle.allPuzzles[puzzleId]
            } else {
                return nil
            }
        }

        pub fun registerPuzzle(
            nftMinter: &NowggNFT.NFTMinter,
            puzzleId: String,
            parentNftTypeId: String,
            childNftTypeIds: [String],
            maxCount: UInt64,
        ) {
            assert(childNftTypeIds.length < 1000, message: "childNftTypeIds must have less than 1000 elements")
            for childPuzzleTypeId in childNftTypeIds {
                nftMinter.registerType(typeId: childPuzzleTypeId, maxCount: maxCount)
            }
            nftMinter.registerType(typeId: parentNftTypeId, maxCount: maxCount)
            NowggPuzzle.allPuzzles[puzzleId] = Puzzle(
                puzzleId: puzzleId,
                parentNftTypeId: parentNftTypeId,
                childNftTypeIds: childNftTypeIds
            )
            emit PuzzleRegistered(
                puzzleId: puzzleId,
                parentNftTypeId: parentNftTypeId,
                childNftTypeIds: childNftTypeIds
            )
        }

        pub fun combinePuzzle(
            nftMinter: &NowggNFT.NFTMinter,
            nftProvider: &{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic, NowggNFT.NowggNFTCollectionPublic},
            puzzleId: String,
            parentNftTypeId: String,
            childNftIds: [UInt64],
            metadata: {String: AnyStruct}
        ) {
            assert(childNftIds.length < 1000, message: "childNftIds must have less than 1000 elements")
            let puzzle = self.borrowPuzzle(puzzleId: puzzleId)!
            let childNftTypes = puzzle.childNftTypeIds

            for nftId in childNftIds {
                let nft = nftProvider.borrowNowggNFT(id: nftId)!
                let metadata = nft.getMetadata()!
                let nftTypeId = metadata["nftTypeId"]! as! String
                assert(childNftTypes.contains(nftTypeId), message: "Incorrect puzzle child NFT provided")

                var index = 0
                for childNftType in childNftTypes {
                    if childNftType == nftTypeId {
                        break
                    }
                    index = index + 1
                }
                childNftTypes.remove(at: index)
            }
            assert(childNftTypes.length == 0, message: "All required puzzle child NFTs not provided")

            nftMinter.mintNFT(recipient: nftProvider, typeId: parentNftTypeId, metaData: metadata)
            for nftId in childNftIds {
                destroy <-nftProvider.withdraw(withdrawID: nftId)
            }

            emit PuzzleCombined(puzzleId: puzzleId, by: nftProvider.owner?.address!)
        }
    }

    init() {
        self.PuzzleHelperStoragePath = /storage/NowggPuzzleHelperStorage
        self.PuzzleHelperPublicPath = /public/NowggPuzzleHelperPublic

        self.allPuzzles = {}

        let puzzleHelper <-create PuzzleHelper()
        self.account.save(<-puzzleHelper, to: self.PuzzleHelperStoragePath)
        self.account.unlink(self.PuzzleHelperPublicPath)
        self.account.link<&NowggPuzzle.PuzzleHelper{NowggPuzzle.PuzzleHelperPublic}>(
            self.PuzzleHelperPublicPath, target: self.PuzzleHelperStoragePath
        )

        emit ContractInitialized()
    }
}
