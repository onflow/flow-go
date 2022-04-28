import MetadataViews from 0x1d7e57aa55817448
import StarlyCollectorScore from 0x5b82f21c0edf76e3
import StarlyMetadataViews from 0x5b82f21c0edf76e3
import StarlyIDParser from 0x5b82f21c0edf76e3

pub contract StarlyMetadata {

    pub struct CollectionMetadata {
        pub let collection: StarlyMetadataViews.Collection
        pub let cards: {UInt32: StarlyMetadataViews.Card}

        init(
            collection: StarlyMetadataViews.Collection,
            cards: {UInt32: StarlyMetadataViews.Card}) {

            self.collection = collection
            self.cards = cards
        }
    }

    access(contract) var metadata: {String: CollectionMetadata}

    pub let AdminStoragePath: StoragePath
    pub let EditorStoragePath: StoragePath
    pub let EditorProxyStoragePath: StoragePath
    pub let EditorProxyPublicPath: PublicPath

    pub fun getViews(): [Type] {
        return [
            Type<MetadataViews.Display>(),
            Type<StarlyMetadataViews.CardEdition>()
        ];
    }

    pub fun resolveView(starlyID: String, view: Type): AnyStruct? {
        switch view {
            case Type<MetadataViews.Display>():
                return self.getDisplay(starlyID: starlyID);
            case Type<StarlyMetadataViews.CardEdition>():
                return self.getCardEdition(starlyID: starlyID);
        }
        return nil;
    }

    pub fun getDisplay(starlyID: String): MetadataViews.Display? {
        if let cardEdition = self.getCardEdition(starlyID: starlyID) {
            let card = cardEdition.card
            let title = card.title
            let edition = cardEdition.edition.toString()
            let editions = card.editions.toString()
            let creatorName = cardEdition.collection.creator.name
            return MetadataViews.Display(
                name: title.concat(" #").concat(edition).concat("/").concat(editions).concat(" by ").concat(creatorName),
                description: cardEdition.card.description,
                thumbnail: MetadataViews.HTTPFile(url: cardEdition.card.mediaSizes[0]!.url)
            )
        }
        return nil
    }

    pub fun getCardEdition(starlyID: String): StarlyMetadataViews.CardEdition? {
        let starlyID = StarlyIDParser.parse(starlyID: starlyID)
        let collectionMetadataOptional = self.metadata[starlyID.collectionID]
        if let collectionMetadata = collectionMetadataOptional {
            let cardOptional = collectionMetadata.cards[starlyID.cardID]
            if let card = cardOptional {
                return StarlyMetadataViews.CardEdition(
                    collection: collectionMetadata.collection,
                    card: card,
                    edition: starlyID.edition,
                    score: StarlyCollectorScore.getCollectorScore(
                        collectionID: starlyID.collectionID,
                        rarity: card.rarity,
                        edition: starlyID.edition,
                        editions: card.editions,
                        priceCoefficient: collectionMetadata.collection.priceCoefficient),
                    url: card.url.concat("/").concat(starlyID.edition.toString()),
                    previewUrl: card.previewUrl.concat("/").concat(starlyID.edition.toString())
                )
            }
        }
        return nil
    }

    pub resource interface IEditor {
        pub fun putCollectionCard(collectionID: String, cardID: UInt32, card: StarlyMetadataViews.Card)
        pub fun putMetadata(collectionID: String, metadata: CollectionMetadata)
        pub fun deleteCollectionCard(collectionID: String, cardID: UInt32)
        pub fun deleteMetadata(collectionID: String)
    }

    pub resource Editor: IEditor {
        pub fun putCollectionCard(collectionID: String, cardID: UInt32, card: StarlyMetadataViews.Card) {
            StarlyMetadata.metadata[collectionID]?.cards?.insert(key: cardID, card)
        }

        pub fun putMetadata(collectionID: String, metadata: CollectionMetadata) {
            StarlyMetadata.metadata.insert(key: collectionID, metadata)
        }

        pub fun deleteCollectionCard(collectionID: String, cardID: UInt32) {
            StarlyMetadata.metadata[collectionID]?.cards?.remove(key: cardID)
        }

        pub fun deleteMetadata(collectionID: String) {
            StarlyMetadata.metadata.remove(key: collectionID)
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

        pub fun putCollectionCard(collectionID: String, cardID: UInt32, card: StarlyMetadataViews.Card) {
            self.editorCapability!.borrow()!
            .putCollectionCard(collectionID: collectionID, cardID: cardID, card: card)
        }

        pub fun putMetadata(collectionID: String, metadata: CollectionMetadata) {
            self.editorCapability!.borrow()!
            .putMetadata(collectionID: collectionID, metadata: metadata)
        }

        pub fun deleteCollectionCard(collectionID: String, cardID: UInt32) {
            self.editorCapability!.borrow()!
            .deleteCollectionCard(collectionID: collectionID, cardID: cardID)
        }

        pub fun deleteMetadata(collectionID: String) {
            self.editorCapability!.borrow()!
            .deleteMetadata(collectionID: collectionID)
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

    init() {
        self.metadata = {}

        self.AdminStoragePath = /storage/starlyMetadataAdmin
        self.EditorStoragePath = /storage/starlyMetadataEditor
        self.EditorProxyPublicPath = /public/starlyMetadataEditorProxy
        self.EditorProxyStoragePath = /storage/starlyMetadataEditorProxy

        let admin <- create Admin()
        let editor <- admin.createNewEditor()
        self.account.save(<-admin, to: self.AdminStoragePath)
        self.account.save(<-editor, to: self.EditorStoragePath)
    }
}
