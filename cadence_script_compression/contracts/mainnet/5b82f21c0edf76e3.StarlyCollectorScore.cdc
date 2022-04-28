pub contract StarlyCollectorScore {

    pub struct Config {
        pub let editions: [[UInt32; 2]]
        pub let rest: UInt32
        pub let last: UInt32

        init(
            editions: [[UInt32; 2]],
            rest: UInt32,
            last: UInt32) {

            self.editions = editions
            self.rest = rest
            self.last = last
        }
    }

    // configs by collection id (or 'default'), then by rarity
    access(contract) let configs: {String: {String: Config}}

    pub let AdminStoragePath: StoragePath
    pub let EditorStoragePath: StoragePath
    pub let EditorProxyStoragePath: StoragePath
    pub let EditorProxyPublicPath: PublicPath

    pub fun getCollectorScore(
        collectionID: String,
        rarity: String,
        edition: UInt32,
        editions: UInt32,
        priceCoefficient: UFix64
    ): UInt32? {
        let collectionConfig = self.configs[collectionID] ?? self.configs["default"] ?? panic("No score config found")
        let rarityConfig = collectionConfig[rarity] ?? panic("No rarity config")

        var editionScore: UInt32 = 0
        if edition == editions && edition != 1 {
            editionScore = rarityConfig.last
        } else {
            for e in rarityConfig.editions {
                if edition <= e[0] {
                    editionScore = e[1]
                    break
                }
            }
        }
        if editionScore == 0 {
            editionScore = rarityConfig.rest
        }

        return UInt32(UFix64(editionScore) * priceCoefficient)
    }

    pub resource interface IEditor {
        pub fun addCollectionConfig(collectionID: String, config: {String: Config})
    }

    pub resource Editor: IEditor {
        pub fun addCollectionConfig(collectionID: String, config: {String: Config}) {
            StarlyCollectorScore.configs[collectionID] = config
        }
    }

    pub resource interface EditorProxyPublic {
        pub fun setEditorCapability(cap: Capability<&Editor>)
    }

    pub resource EditorProxy: IEditor, EditorProxyPublic {
        access(self) var editorCapability: Capability<&Editor>?

        pub fun setEditorCapability(cap: Capability<&Editor>) {
            self.editorCapability = cap
        }

        pub fun addCollectionConfig(collectionID: String, config: {String: Config}) {
            self.editorCapability!.borrow()!
            .addCollectionConfig(collectionID: collectionID, config: config)
        }

        init() {
            self.editorCapability = nil
        }
    }

    pub fun createEditorProxy(): @EditorProxy {
        return <- create EditorProxy()
    }

    pub resource Admin {
        pub fun createNewEditor(): @Editor {
            return <- create Editor()
        }
    }

    init () {
        self.AdminStoragePath = /storage/starlyCollectorScoreAdmin
        self.EditorStoragePath = /storage/starlyCollectorScoreEditor
        self.EditorProxyPublicPath = /public/starlyCollectorScoreEditorProxy
        self.EditorProxyStoragePath = /storage/starlyCollectorScoreEditorProxy

        let admin <- create Admin()
        let editor <- admin.createNewEditor()
        self.account.save(<-admin, to: self.AdminStoragePath)
        self.account.save(<-editor, to: self.EditorStoragePath)

        self.configs = {}
    }
}
